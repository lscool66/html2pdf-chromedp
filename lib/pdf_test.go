package lib

import (
	"fmt"
	"html2pdf/tests"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test_Covert(t *testing.T) {
	files, err := filepath.Glob(tests.GetLocalPath("../tests/*"))
	if err != nil {
		t.Log(err)
		t.Fail()
		return
	}

	files = Filter(files, func(item string) bool {
		ext := filepath.Ext(item)

		if strings.EqualFold(ext, ".jpg") {
			return true
		}
		if strings.EqualFold(ext, ".png") {
			return true
		}
		if strings.EqualFold(ext, ".gif") {
			return true
		}
		if strings.EqualFold(ext, ".jpeg") {
			return true
		}

		return false
	})

	for _, file := range files {
		dest_file := file + ".pdf"
		err = ConvertToPdf(file, dest_file)
		if err != nil {
			t.Log(err)
			t.Fail()
			return
		}
		t.Log(dest_file)
		defer os.Remove(dest_file)
	}

	t.Log("PASS")
}

func Test_Combine(t *testing.T) {
	files, err := filepath.Glob(tests.GetLocalPath("../tests/*.pdf"))
	if err != nil {
		t.Log(err)
		t.Fail()
		return
	}

	dest_file := tests.GetLocalPath("../tests/bundle.pdf")

	if _, err := os.Stat(dest_file); err == nil {
		os.Remove(dest_file)
	}

	err = CombinePDF(files, dest_file)
	if err != nil {
		t.Log(err)
		t.Fail()
		return
	}
	t.Log(dest_file)
	defer os.Remove(dest_file)

	t.Log("PASS")
}

func TestGetMetaData(t *testing.T) {
	pdfPath := os.Getenv("TEST_PDF_PATH")
	meta, err := GetMetaData(pdfPath)
	if err != nil {
		t.Log(err)
		t.Fail()
		return
	}

	t.Log(tests.ToJSON(meta))
}

func TestSetPDFMetaData(t *testing.T) {
	pdfPath := os.Getenv("TEST_PDF_PATH")
	meta := &PDFMetaInfo{
		Author:      "driver.com.hk",
		Creator:     "HTML2PDF",
		Keywords:    "motors policy",
		Subject:     "PDF Document",
		Title:       "Cover Note",
	}

	err := SetPDFMetaData(pdfPath, meta)
	if err != nil {
		t.Log(err)
		t.Fail()
		return
	}
}

func TestEncryptPDF(t *testing.T) {
	pdfPath := os.Getenv("TEST_PDF_PATH")
	err := EncryptPDF(pdfPath, fmt.Sprintf("%s.encrypted.pdf", pdfPath), "driver.com.hk")
	if err != nil {
		t.Log(err)
		t.Fail()
		return
	}
}