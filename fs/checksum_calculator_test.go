package fs_test

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/packit/fs"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testChecksumCalculator(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		calculator fs.ChecksumCalculator
		workingDir string
	)

	context("Sum", func() {
		it.Before(func() {
			var err error
			workingDir, err = os.MkdirTemp("", "working-dir")
			Expect(err).NotTo(HaveOccurred())

			calculator = fs.NewChecksumCalculator()
		})

		it.After(func() {
			Expect(os.RemoveAll(workingDir)).To(Succeed())
		})

		context("when given a single file", func() {
			var path string

			it.Before(func() {
				path = filepath.Join(workingDir, "some-file")
				Expect(os.WriteFile(path, []byte{}, os.ModePerm)).To(Succeed())
			})

			it("generates the SHA256 checksum for that file", func() {
				sum, err := calculator.Sum(path)
				Expect(err).ToNot(HaveOccurred())
				Expect(sum).To(Equal("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"))
			})
		})

		context("when given an empty directory", func() {
			var path string
			it.Before(func() {
				path = filepath.Join(workingDir, "some-dir")
				Expect(os.MkdirAll(path, os.ModePerm)).To(Succeed())
			})

			it("generates the SHA256 checksum for that directory", func() {
				sum, err := calculator.Sum(path)
				Expect(err).ToNot(HaveOccurred())
				Expect(sum).To(Equal("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"))
			})

		})

		context("when given multiple files", func() {
			var path1 string
			var path2 string
			var paths []string

			it.Before(func() {
				path1 = filepath.Join(workingDir, "some-file1")
				Expect(os.WriteFile(path1, []byte{}, os.ModePerm)).To(Succeed())
				path2 = filepath.Join(workingDir, "some-file2")
				Expect(os.WriteFile(path2, []byte{}, os.ModePerm)).To(Succeed())
			})

			it("generates the SHA256 checksum for multiple files", func() {
				sum, err := calculator.Sum(path1, path2)
				Expect(err).ToNot(HaveOccurred())
				Expect(sum).To(Equal("2dba5dbc339e7316aea2683faf839c1b7b1ee2313db792112588118df066aa35"))
			})

			context("when the files are different", func() {
				it.Before(func() {
					// Generate a bunch of files and shuffle input order to ensure test fails consistently without sorting implemented
					for i := 0; i < 10; i++ {
						path := filepath.Join(workingDir, fmt.Sprintf("some-file-%d", i))
						Expect(os.WriteFile(path, []byte(fmt.Sprintf("some-file-contents-%d", i)), os.ModePerm)).To(Succeed())
						paths = append(paths, path)
					}
				})

				it("generates the same checksum no matter the order of the inputs", func() {
					var shuffledPaths []string
					sum1, err := calculator.Sum(paths...)
					Expect(err).NotTo(HaveOccurred())

					for _, value := range rand.Perm(len(paths)) {
						shuffledPaths = append(shuffledPaths, paths[value])
					}

					calculator2 := fs.NewChecksumCalculator()
					sum2, err := calculator2.Sum(shuffledPaths...)
					Expect(err).NotTo(HaveOccurred())
					Expect(sum1).To(Equal(sum2))
				})
			})

			context("failure cases", func() {
				context("either of the files cannot be read", func() {
					it.Before(func() {
						Expect(os.Chmod(path2, 0222)).To(Succeed())
					})

					it("returns an error", func() {
						_, err := calculator.Sum(path1, path2)
						Expect(err).To(MatchError(ContainSubstring("failed to calculate checksum")))
						Expect(err).To(MatchError(ContainSubstring("permission denied")))
					})
				})
			})
		})

		context("when given multiple directories", func() {
			var dir1 string
			var dir2 string

			it.Before(func() {
				dir1 = filepath.Join(workingDir, "some-dir")
				Expect(os.MkdirAll(dir1, os.ModePerm)).To(Succeed())

				dir2 = filepath.Join(workingDir, "some-other-dir")
				Expect(os.MkdirAll(dir2, os.ModePerm)).To(Succeed())

				Expect(os.WriteFile(filepath.Join(dir1, "some-file"), []byte{}, os.ModePerm)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(dir1, "some-other-file"), []byte{}, os.ModePerm)).To(Succeed())

				Expect(os.WriteFile(filepath.Join(dir2, "some-file"), []byte{}, os.ModePerm)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(dir2, "some-other-file"), []byte{}, os.ModePerm)).To(Succeed())
			})

			it("returns the 256 sha sum of a directory containing the directories", func() {
				sum, err := calculator.Sum(dir1, dir2)
				Expect(err).ToNot(HaveOccurred())
				Expect(sum).To(Equal("9fb03d22515ca48e57b578de80bbc1e75d5126dbb2de6db177947c3da3b2276f"))
			})

			context("failure cases", func() {
				context("when one of the directories cannot be read", func() {
					it.Before(func() {
						Expect(os.Chmod(dir2, 0222)).To(Succeed())
					})

					it.After(func() {
						Expect(os.Chmod(dir2, os.ModePerm)).To(Succeed())
					})

					it("returns an error", func() {
						_, err := calculator.Sum(dir1, dir2)
						Expect(err).To(MatchError(ContainSubstring("failed to calculate checksum")))
						Expect(err).To(MatchError(ContainSubstring("permission denied")))
					})
				})

				context("when a file in the directory cannot be read", func() {
					it.Before(func() {
						Expect(os.Chmod(filepath.Join(dir2, "some-file"), 0222)).To(Succeed())
					})

					it("returns an error", func() {
						_, err := calculator.Sum(dir1, dir2)
						Expect(err).To(MatchError(ContainSubstring("failed to calculate checksum")))
						Expect(err).To(MatchError(ContainSubstring("permission denied")))
					})
				})
			})
		})

		context("when given a combination of files and directories", func() {
			var dir string
			var path string

			it.Before(func() {
				dir = filepath.Join(workingDir, "some-dir")
				path = filepath.Join(workingDir, "some-filepath")
				Expect(os.MkdirAll(dir, os.ModePerm)).To(Succeed())

				Expect(os.WriteFile(filepath.Join(dir, "some-file"), []byte{}, os.ModePerm)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(dir, "some-other-file"), []byte{}, os.ModePerm)).To(Succeed())
				Expect(os.WriteFile(path, []byte{}, os.ModePerm)).To(Succeed())
			})

			it("returns the 256 sha sum of a directory containing the file and directory", func() {
				sum, err := calculator.Sum(dir, path)
				Expect(err).ToNot(HaveOccurred())
				Expect(sum).To(Equal("1a03c02fb531d7e1ce353b2f20711c79af2b66730d6de865fb130734973ccd2c"))
			})
		})

		context("when given multiple items with same base name", func() {
			var dir string
			var path string

			it.Before(func() {
				dir = filepath.Join(workingDir, "some-dir", "the-item")
				path = filepath.Join(workingDir, "the-item")
				Expect(os.MkdirAll(dir, os.ModePerm)).To(Succeed())

				Expect(os.WriteFile(filepath.Join(dir, "some-file"), []byte{}, os.ModePerm)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(dir, "some-other-file"), []byte{}, os.ModePerm)).To(Succeed())
				Expect(os.WriteFile(path, []byte{}, os.ModePerm)).To(Succeed())

			})

			it("returns the 256 sha sum of the items", func() {
				sum, err := calculator.Sum(dir, path)
				Expect(err).ToNot(HaveOccurred())
				Expect(sum).To(Equal("1a03c02fb531d7e1ce353b2f20711c79af2b66730d6de865fb130734973ccd2c"))
			})
		})

		context("failure cases", func() {
			context("when any of the given paths do not exist", func() {
				it("returns an error", func() {
					_, err := calculator.Sum("not a real path")
					Expect(err).To(MatchError(ContainSubstring("failed to calculate checksum")))
					Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
				})
			})
		})
	})
}
