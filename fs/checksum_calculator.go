package fs

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
)

// ChecksumCalculator is used to calculate the SHA256 checksum of a given set of
// files and/or directory trees and additional byte arrays that are provided.
// When multiple files are given or are present in a directory tree the SHA256
// calculation will be performed in parallel.
type ChecksumCalculator struct {
	sumValue []byte
	hash     hash.Hash
}

// NewChecksumCalculator returns a new instance of a ChecksumCalculator.
func NewChecksumCalculator() ChecksumCalculator {
	return ChecksumCalculator{
		hash: sha256.New(),
	}
}

type calculatedFile struct {
	path     string
	checksum []byte
	err      error
}

// SumAsHexString returns the sum of the underlying SHA256 hash in hex format.
// Additional calls to the `Write(p []byte)` and `WritePath(paths ...strings)`
// methods update the underlying SHA265 value of `ChecksumCalculator` so
// `SumAsHexString()` can be called once or after any additional data
// is written to ChecksumCalculator.
func (c ChecksumCalculator) SumAsHexString() string {
	return hex.EncodeToString(c.sumValue)
}

// Write a byte slice onto an internal SHA265 hash object so that it can be summed later.
// This method can be called multiple times and interleaved with calls to
// `WritePaths((paths ...string)`
func (c ChecksumCalculator) Write(p []byte) (int, error) {
	n, err := c.hash.Write(p)
	if err != nil {
		return n, err
	}
	return n, err
}

// WritePaths adds intermediate checksum values generated in parallel from a given
// set of files and/or files in a set of directoriy trees to be summed later.
// Can be interleved with calls to `Write(p []byte)`
func (c *ChecksumCalculator) WritePaths(paths ...string) error {
	var files []string
	for _, path := range paths {
		err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.Mode().IsRegular() {
				files = append(files, path)
			}

			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to calculate checksum: %w", err)
		}
	}

	//Gather all checksums
	var sums [][]byte
	for _, f := range getParallelChecksums(files) {
		if f.err != nil {
			return fmt.Errorf("failed to calculate checksum: %w", f.err)
		}

		sums = append(sums, f.checksum)
	}

	if len(sums) == 1 {
		c.sumValue = sums[0]
		return nil
	}

	for _, sum := range sums {
		_, err := c.hash.Write(sum)
		if err != nil {
			return fmt.Errorf("failed to calculate checksum: %w", err)
		}
	}

	c.sumValue = c.hash.Sum(nil)
	return nil
}

// Sum returns a hex-encoded SHA256 checksum as a string for files and/or the files in a
// set of directory trees.
//
// This method remains for backward compatibiliy purposes.
func (c ChecksumCalculator) Sum(paths ...string) (string, error) {
	err := c.WritePaths(paths...)
	if err != nil {
		return "", err
	}
	return c.SumAsHexString(), nil
}

func getParallelChecksums(filesFromDir []string) []calculatedFile {
	var checksumResults []calculatedFile
	numFiles := len(filesFromDir)
	files := make(chan string, numFiles)
	calculatedFiles := make(chan calculatedFile, numFiles)

	//Spawns workers
	for i := 0; i < runtime.NumCPU(); i++ {
		go fileChecksumer(files, calculatedFiles)
	}

	//Puts files in worker queue
	for _, f := range filesFromDir {
		files <- f
	}

	close(files)

	//Pull all calculated files off of result queue
	for i := 0; i < numFiles; i++ {
		checksumResults = append(checksumResults, <-calculatedFiles)
	}

	//Sort calculated files for consistent checksuming
	sort.Slice(checksumResults, func(i, j int) bool {
		return checksumResults[i].path < checksumResults[j].path
	})

	return checksumResults
}

func fileChecksumer(files chan string, calculatedFiles chan calculatedFile) {
	for path := range files {
		result := calculatedFile{path: path}

		file, err := os.Open(path)
		if err != nil {
			result.err = err
			calculatedFiles <- result
			continue
		}

		hash := sha256.New()
		_, err = io.Copy(hash, file)
		if err != nil {
			result.err = err
			calculatedFiles <- result
			continue
		}

		if err := file.Close(); err != nil {
			result.err = err
			calculatedFiles <- result
			continue
		}

		result.checksum = hash.Sum(nil)
		calculatedFiles <- result
	}
}
