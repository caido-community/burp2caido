package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConverter(t *testing.T) {
	// Create temporary directory for test databases
	tmpDir, err := os.MkdirTemp("", "caido_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Copy test databases from testdata to temp directory
	files := []string{"database.caido", "database_raw.caido"}
	for _, file := range files {
		src := filepath.Join("testdata", file)
		dst := filepath.Join(tmpDir, file)
		if err := copyFile(src, dst); err != nil {
			t.Fatal(err)
		}
	}

	// Create converter
	converter, err := NewConverter(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer converter.Close()

	// Test conversion
	err = converter.ConvertBurpFile(filepath.Join("testdata", "burp_export.xml"))
	if err != nil {
		t.Fatal(err)
	}

	// TODO: Add assertions here to verify the conversion was successful
}

func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0644)
}
