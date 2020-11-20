package fs

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sort"
)

// ChecksumCalculator can be used to calculate the SHA256 checksum of a given file or
// directory. When given a directory, checksum calculation will be performed in
// parallel.
type ChecksumCalculator struct{}

// NewChecksumCalculator returns a new instance of a ChecksumCalculator.
func NewChecksumCalculator() ChecksumCalculator {
	return ChecksumCalculator{}
}

type calculatedFile struct {
	path     string
	checksum []byte
	err      error
}

// SumMultiple returns a hex-encoded SHA256 checksum value of a set of files or
// directories given a path.
func (c ChecksumCalculator) SumMultiple(paths ...string) (shasum string, err error) {
	tempDir, err := ioutil.TempDir("", "checksum*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	// allows checking error of deferred RemoveAll
	defer func() {
		if e := os.RemoveAll(tempDir); e != nil {
			err = e
		}
	}()

	for _, path := range paths {
		randBytes := make([]byte, 16)
		_, err := rand.Read(randBytes)
		if err != nil {
			return "", fmt.Errorf("failed to generate random filename: %w", err)
		}

		err = Copy(path, filepath.Join(tempDir, hex.EncodeToString(randBytes)))
		if err != nil {
			return "", fmt.Errorf("failed to calculate checksum: %w", err)
		}
	}
	return c.Sum(tempDir)
}

// Sum returns a hex-encoded SHA256 checksum value of a file or directory given a path.
func (c ChecksumCalculator) Sum(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("failed to calculate checksum: %w", err)
	}

	if !info.IsDir() {
		file, err := os.Open(path)
		if err != nil {
			return "", fmt.Errorf("failed to calculate checksum: %w", err)
		}
		defer file.Close()

		hash := sha256.New()
		_, err = io.Copy(hash, file)
		if err != nil {
			return "", fmt.Errorf("failed to calculate checksum: %w", err)
		}

		return hex.EncodeToString(hash.Sum(nil)), nil
	}

	//Finds all files in directoy
	var filesFromDir []string
	err = filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.Mode().IsRegular() {
			filesFromDir = append(filesFromDir, path)
		}

		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to calculate checksum: %w", err)
	}

	//Gather all checksums into one byte array and check for checksum calculation errors
	hash := sha256.New()
	for _, f := range getParallelChecksums(filesFromDir) {
		if f.err != nil {
			return "", fmt.Errorf("failed to calculate checksum: %w", f.err)
		}

		_, err := hash.Write(f.checksum)
		if err != nil {
			return "", fmt.Errorf("failed to calculate checksum: %w", err)
		}
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
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
