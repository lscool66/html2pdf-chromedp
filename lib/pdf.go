package lib

import (
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"image"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"github.com/jung-kurt/gofpdf"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func Filter(vs []string, f func(string) bool) []string {
	vsf := make([]string, 0)
	for _, v := range vs {
		if f(v) {
			vsf = append(vsf, v)
		}
	}
	return vsf
}

func ConvertToPdf(src_image_path string, dest_pdf_path string) error {
	Log.Debugf("ConvertToPdf: %s => %s\n", src_image_path, dest_pdf_path)
	src_image, err := os.Open(src_image_path)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer src_image.Close()
	img, _, err := image.Decode(src_image)
	if err != nil {
		Log.Error(err)
		return err
	}
	rect := img.Bounds()

	pdfTpye := "P"
	if rect.Dx() > rect.Dy() {
		pdfTpye = "L"
	}

	pdf := gofpdf.New(pdfTpye, "mm", "A4", ".")
	pdf.AddPage()
	w, _ := pdf.GetPageSize()
	pdf.Image(src_image_path, 0, 0, w, 0, false, "", 0, "")
	err = pdf.OutputFileAndClose(dest_pdf_path)
	if err != nil {
		Log.Error(err)
		return err
	}

	return nil
}

func CombinePDF(files []string, dest_pdf_path string) error {
	Log.Debugf("CombinePDF: %v => %s\n", files, dest_pdf_path)
	//处理合并过程中可能出现的异常
	defer func() {
		if err := recover(); err != nil {
			Log.Error(err)
		}
	}()

	config := model.NewDefaultConfiguration()
	config.ValidationMode = model.ValidationRelaxed
	err := api.MergeCreateFile(files, dest_pdf_path, true, config)
	if err != nil {
		Log.Error(err)
		return err
	}

	return nil
}

type PDFMetaInfo struct {
	Title       string `json:"title"`
	Author      string `json:"author"`
	Subject     string `json:"subject"`
	Keywords    string `json:"keywords"`
	Creator     string `json:"creator"`
	Producer   string `json:"producer"`
}

func GetMetaData(src_pdf_path string) (*PDFMetaInfo, error) {
	ctx, err := api.ReadContextFile(src_pdf_path)
	if err != nil {
		Log.Error(err)
		return nil, err
	}

	return &PDFMetaInfo{
		Title:       ctx.Title,
		Author:      ctx.Author,
		Subject:     ctx.Subject,
		Keywords:    ctx.Keywords,
		Creator:     ctx.Creator,
		Producer: ctx.Producer,
	}, nil
}

func SetPDFMetaData(src_pdf_path string, meta *PDFMetaInfo) error {
	ctx, err := api.ReadContextFile(src_pdf_path)
	if err != nil {
		Log.Error(err)
		return err
	}

	mappings := make(map[string]string)
	keys := []string{}

	if meta.Title != "" {
		mappings["Title"] = meta.Title
		keys = append(keys, "Title")
	}
	if meta.Author != "" {
		mappings["Author"] = meta.Author
		keys = append(keys, "Author")
	}
	if meta.Subject != "" {
		mappings["Subject"] = meta.Subject
		keys = append(keys, "Subject")
	}
	if meta.Keywords != "" {
		mappings["Keywords"] = meta.Keywords
		keys = append(keys, "Keywords")
	}
	if meta.Creator != "" {
		mappings["Creator"] = meta.Creator
		keys = append(keys, "Creator")
	}
	if meta.Producer != "" {
		mappings["Producer"] = meta.Producer
		keys = append(keys, "Producer")
	}

	_, err = pdfcpu.PropertiesRemove(ctx, keys)
	if err != nil {
		Log.Error(err)
		return err
	}

	err = pdfcpu.PropertiesAdd(ctx, mappings)
	if err != nil {
		Log.Error(err)
		return err
	}

	return api.WriteContextFile(ctx, src_pdf_path)
}
