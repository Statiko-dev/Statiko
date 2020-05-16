/*
Copyright © 2020 Alessandro Segala (@ItalyPaleAle)

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, version 3 of the License.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package utils

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/mholt/archiver"
)

// Type of archive
const (
	ArchiveTar = iota
	ArchiveTarBz2
	ArchiveTarGz
	ArchiveTarLz4
	ArchiveTarSz
	ArchiveTarXz
	ArchiveZip
	ArchiveRar
)

// Valid extensions
var ArchiveExtensions = []string{".zip", ".tar", ".tar.bz2", ".tbz2", ".tar.gz", ".tgz", ".tar.lz4", ".tlz4", ".tar.sz", ".tsz", ".tar.xz", ".txz", ".rar"}

// Used as interface
type archiveHeader struct {
	Name string
}

// ArchiveTypeByExtension returns the type of the archive by the file's extension
// Note: this function assumes that the file was already validated to contain one of the valid extensions
// Ensure to use the SiteApp.Validate() method first
func ArchiveTypeByExtension(name string) int {
	// Get the last 4 characters
	// This is safe since we know the extension is one of those in the list
	name = strings.ToLower(name)
	switch name[len(name)-4:] {
	case ".tar":
		return ArchiveTar
	case ".bz2", "tbz2":
		return ArchiveTarBz2
	case "r.gz", ".tgz":
		return ArchiveTarGz
	case ".lz4", "tlz4":
		return ArchiveTarLz4
	case "r.sz", ".tsz":
		return ArchiveTarSz
	case "r.xz", ".txz":
		return ArchiveTarXz
	case ".zip":
		return ArchiveZip
	case ".rar":
		return ArchiveRar
	}

	return -1
}

// ExtractArchive extracts a compressed archive
// Reads input from a stream in, and extracts all files into dst
// This supports all archive formats supported by archiver, including zip, tar.gz, tar.bz2, rar
func ExtractArchive(dst string, in io.Reader, size int64, format int) error {
	// Open the archive, depending on the format
	var ar archiver.Reader
	switch format {
	case ArchiveTar:
		ar = archiver.NewTar()
	case ArchiveTarBz2:
		ar = archiver.NewTarBz2()
	case ArchiveTarGz:
		ar = archiver.NewTarGz()
	case ArchiveTarLz4:
		ar = archiver.NewTarLz4()
	case ArchiveTarSz:
		ar = archiver.NewTarSz()
	case ArchiveTarXz:
		ar = archiver.NewTarXz()
	case ArchiveZip:
		ar = archiver.NewZip()
	case ArchiveRar:
		ar = archiver.NewRar()
	default:
		return errors.New("invalid archive format")
	}
	err := ar.Open(in, size)
	if err != nil {
		return err
	}
	defer ar.Close()

	// Iterate through all element
	for {
		// Open the element
		// We won't defer the call to close, or it will happen at the end of all files
		f, err := ar.Read()
		if err == io.EOF {
			// EOF means we've read all elements in the archive
			break
		}
		if err != nil {
			return fmt.Errorf("error opening file: %v", err)
		}

		// Extract the element
		err = extractElement(dst, &f, format)
		if err != nil {
			f.Close()
			return fmt.Errorf("error extracting file: %v", err)
		}

		// Close the stream
		f.Close()
	}

	return nil
}

// Extracts each element from the archive
func extractElement(dst string, f *archiver.File, format int) error {
	// If the header is nil, skip the element
	if f.Header == nil {
		return nil
	}

	// Get the file name
	// f.FileInfo.Name() is not reliable
	val := reflect.ValueOf(f.Header)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		// In case of error, print a warning and skip the file
		logger.Println("[Warn] ExtractArchive: item's header is not struct, but rather", val.Kind())
		return nil
	}
	nameField := val.FieldByName("Name")
	if !nameField.IsValid() || nameField.Kind() != reflect.String {
		// In case of error, print a warning and skip the file
		logger.Println("[Warn] ExtractArchive: item's header.Name is invalid")
		return nil
	}
	name := nameField.String()

	// Ignore paths that start with __MACOSX, which are used by macOS to store metadata in zip files
	if strings.HasPrefix(name, "__MACOSX") {
		return nil
	}

	// Get the path relative to the destination
	target := filepath.Join(dst, name)
	if target == "" || target == string(os.PathSeparator) {
		return errors.New("target is empty")
	}
	if !extractPathValid(target, dst) {
		logger.Println("[Warn] caught potential zip-slip attack with file", target)
		return fmt.Errorf("illegal target: %s", target)
	}

	// Folders
	f.FileInfo.Name()
	if f.IsDir() {
		err := EnsureFolder(target)
		if err != nil {
			return err
		}
	} else if isRegularFile(f, format) {
		// Ensure the parent directory exists
		dir := filepath.Dir(target)
		err := EnsureFolder(dir)
		if err != nil {
			return err
		}

		// Write the file
		mode := f.Mode()
		if mode == 0 {
			mode = 0644
		}
		// Do not call defer on Close(), or that will make all files to stay open until the loop is over
		out, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(mode))
		if err != nil {
			return err
		}

		// Copy the file's contents
		_, err = io.Copy(out, f)
		if err != nil {
			out.Close()
			return err
		}

		out.Close()
	} else {
		// Skip the file
		logger.Println("[Warn] extractElement: unsupported element type")
	}

	return nil
}

// Checks if a file is a regular file
// This should be executed after checking for IsDir()
func isRegularFile(f *archiver.File, format int) bool {
	switch format {
	case ArchiveTar, ArchiveTarBz2, ArchiveTarGz, ArchiveTarLz4, ArchiveTarSz, ArchiveTarXz:
		h, ok := f.Header.(*tar.Header)
		if !ok {
			logger.Println("[Warn] isRegularFile: expected header to be tar.Header, but casting failed")
			return false
		}
		return h.Typeflag == tar.TypeReg ||
			h.Typeflag == tar.TypeRegA ||
			h.Typeflag == tar.TypeChar ||
			h.Typeflag == tar.TypeBlock ||
			h.Typeflag == tar.TypeFifo ||
			h.Typeflag == tar.TypeGNUSparse
	case ArchiveZip, ArchiveRar:
		return !f.IsDir()
	}

	return false
}

// Sanitize the path where to extract files
// Prevents zip-slip vulnerabilities
// Adapted from: https://snyk.io/research/zip-slip-vulnerability
func extractPathValid(filePath string, destination string) bool {
	destpath := filepath.Join(destination, filePath)
	return strings.HasPrefix(destpath, filepath.Clean(destination)+string(os.PathSeparator))
}
