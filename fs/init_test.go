package fs_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestUnitFS(t *testing.T) {
	suite := spec.New("packit/fs", spec.Report(report.Terminal{}))
	suite("ChecksumCalculator", testChecksumCalculator)
	suite("Copy", testCopy)
	suite("FileExists", testFileExists)
	suite("IsEmptyDir", testIsEmptyDir)
	suite("Move", testMove)
	suite.Run(t)
}
