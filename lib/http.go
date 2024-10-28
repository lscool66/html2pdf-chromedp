package lib

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

type HTTPService struct {
	config *Config
}

func NewHTTP(conf *Config) *HTTPService {
	return &HTTPService{
		config: conf,
	}
}

func (s *HTTPService) Start() {
	r := mux.NewRouter()
	r.HandleFunc("/", s.RedirectSwagger)
	r.HandleFunc("/htmlpdf", s.HTMLPDF)
	r.HandleFunc("/linkpdf", s.LINKPDF)
	// 适配cwinlis
	if s.config.PatchCwinlis {
		r.HandleFunc("/pdf/v1/api/pdf/exporter/generatePdf", s.LINKPDF)
	}
	r.HandleFunc("/combine", s.COMBINE)
	r.HandleFunc("/link/combine", s.LinkCombine)
	r.PathPrefix("/swagger/").Handler(http.StripPrefix("/swagger/",
		http.FileServer(http.Dir(fmt.Sprintf("%s/swagger", s.config.WebRoot)))))
	r.NotFoundHandler = http.HandlerFunc(s.NotFoundHandle)

	Log.Info("http service starting")
	Log.Infof("Please open http://%s\n", s.config.Listen)
	http.ListenAndServe(s.config.Listen, r)
}

func (s *HTTPService) NotFoundHandle(writer http.ResponseWriter, request *http.Request) {
	http.Error(writer, "handle not found!", 404)
}

func (s *HTTPService) RedirectSwagger(writer http.ResponseWriter, request *http.Request) {
	http.Redirect(writer, request, "/swagger/index.html", http.StatusMovedPermanently)
}

func (s *HTTPService) HTMLPDF(writer http.ResponseWriter, request *http.Request) {

	upload_text := request.FormValue("upload")
	var bin []byte
	if len(upload_text) > 0 {
		bin = []byte(upload_text)
	} else {
		request.ParseMultipartForm(32 << 20)
		file, _, err := request.FormFile("upload")
		if err != nil {
			Log.Error(err)
			http.Error(writer, err.Error(), 500)
			return
		}
		defer file.Close()

		bin, err = io.ReadAll(file)
		if err != nil {
			Log.Error(err)
			http.Error(writer, err.Error(), 500)
			return
		}
	}

	htmlpdf := NewHTMLPDF(s.config)
	file, err := htmlpdf.BuildFromSource(bin)
	if err != nil {
		Log.Error(err)
		http.Error(writer, err.Error(), 500)
		return
	}
	// writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%d.pdf", time.Now().UnixNano()))
	writer.Header().Set("Content-Type", "application/pdf")

	err = SetPDFMetaData(file, &PDFMetaInfo{
		Author:   s.config.BuildMeta.Author,
		Creator:  s.config.BuildMeta.Creator,
		Keywords: s.config.BuildMeta.Keywords,
		Subject:  s.config.BuildMeta.Subject,
	})
	if err != nil {
		Log.Error(err)
	}

	pdf, err := os.Open(file)
	if err != nil {
		http.Error(writer, err.Error(), 500)
		return
	}
	defer pdf.Close()
	defer time.AfterFunc(time.Second*10, func() {
		os.Remove(file)
	})
	_, err = io.Copy(writer, pdf)
	if err != nil {
		http.Error(writer, err.Error(), 500)
		return
	}
}

func (s *HTTPService) LINKPDF(writer http.ResponseWriter, request *http.Request) {
	link := request.FormValue("link")
	// 适配cwinlis
	var papperOption *PapperOption
	papperOption_json := request.FormValue("papperOption")
	if s.config.PatchCwinlis {
		hospCode := request.FormValue("hospCode")
		reportId := request.FormValue("reportId")
		link = fmt.Sprintf("%s?hospCode=%s&reportId=%s", s.config.CwinlisUrl, hospCode, reportId)

		json.Unmarshal([]byte(papperOption_json), &papperOption)
	}

	htmlpdf := NewHTMLPDF(s.config)
	// 适配cwinlis
	if papperOption != nil {
		if papperOption.Scale > 0 {
			htmlpdf.pdfOption.Scale = papperOption.Scale
		}
		var paperSize PaperSize
		switch papperOption.PaperFormat {
		case "A0":
			paperSize = A0
		case "A1":
			paperSize = A1
		case "A2":
			paperSize = A2
		case "A3":
			paperSize = A3
		case "A4":
			paperSize = A4
		case "A5":
			paperSize = A5
		case "A6":
			paperSize = A6
		case "LETTER":
			paperSize = LETTER
		case "LEGAL":
			paperSize = LEGAL
		case "TABLOID":
			paperSize = TABLOID
		case "LEDGER":
			paperSize = LEDGER
		default:
			http.Error(writer, "Invalid paper type", http.StatusBadRequest)
			return
		}
		if paperSize != A4 {
			htmlpdf.pdfOption.PaperHeight = paperSize.getHeight()
			htmlpdf.pdfOption.PaperWidth = paperSize.getWidth()
		}
		htmlpdf.pdfOption.Landscape = papperOption.Landscape
		htmlpdf.pdfOption.PreferCSSPageSize = papperOption.PreferCSSPageSize
		if papperOption.PagerMargin.Top > 0 {
			htmlpdf.pdfOption.MarginTop = papperOption.PagerMargin.Top
		}
		if papperOption.PagerMargin.Left > 0 {
			htmlpdf.pdfOption.MarginLeft = papperOption.PagerMargin.Left
		}
		if papperOption.PagerMargin.Right > 0 {
			htmlpdf.pdfOption.MarginRight = papperOption.PagerMargin.Right
		}
		if papperOption.PagerMargin.Bottom > 0 {
			htmlpdf.pdfOption.MarginBottom = papperOption.PagerMargin.Bottom
		}
		if papperOption.Width > 0 {
			htmlpdf.pdfOption.PaperWidth = papperOption.Width
		}
		if papperOption.Height > 0 {
			htmlpdf.pdfOption.PaperHeight = papperOption.Height
		}

	}
	file, err := htmlpdf.BuildFromLink(link)
	if err != nil {
		Log.Error(err)
		http.Error(writer, err.Error(), 500)
		return
	}
	// writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%d.pdf", time.Now().UnixNano()))
	writer.Header().Set("Content-Type", "application/pdf")

	err = SetPDFMetaData(file, &PDFMetaInfo{
		Author:   s.config.BuildMeta.Author,
		Creator:  s.config.BuildMeta.Creator,
		Keywords: s.config.BuildMeta.Keywords,
		Subject:  s.config.BuildMeta.Subject,
	})
	if err != nil {
		Log.Error(err)
	}

	pdf, err := os.Open(file)
	if err != nil {
		http.Error(writer, err.Error(), 500)
		return
	}
	defer pdf.Close()
	defer time.AfterFunc(time.Second*10, func() {
		os.Remove(file)
	})
	_, err = io.Copy(writer, pdf)
	if err != nil {
		http.Error(writer, err.Error(), 500)
		return
	}
}

func (s *HTTPService) LinkCombine(writer http.ResponseWriter, request *http.Request) {
	if err := request.ParseForm(); err != nil {
		http.Error(writer, err.Error(), 500)
	}

	input_files := make([]string, 0)
	for key, values := range request.PostForm {
		if strings.EqualFold(key, "file") {
			input_files = append(input_files, values...)
		}
	}

	downloader := NewDownloader(input_files, s.config)
	queue := downloader.GetDownloadedFiles()
	localFiles := make([]string, 0)
	worker := make(chan bool, s.config.Worker)
	defer close(worker)

	var wg sync.WaitGroup
	for item := range queue {

		if item.IsPDF() {
			localFiles = append(localFiles, item.LocalPath)
			continue
		}
		if item.IsImage() {
			defer os.Remove(item.LocalPath)
			wg.Add(1)

			go func(job *JobItem, wg *sync.WaitGroup) {
				defer wg.Done()
				worker <- true
				defer func() {
					<-worker
				}()

				Log.Debug("convert image to pdf")

				pdf_path := fmt.Sprintf("%s.pdf", item.LocalPath)
				err := ConvertToPdf(item.LocalPath, pdf_path)
				if err != nil {
					Log.Error(err)
					return
				}
				localFiles = append(localFiles, pdf_path)
			}(item, &wg)

			continue
		}
		if item.IsHTML() {
			defer os.Remove(item.LocalPath)
			wg.Add(1)

			go func(job *JobItem, wg *sync.WaitGroup) {
				defer wg.Done()
				worker <- true
				defer func() {
					<-worker
				}()

				htmlpdf := NewHTMLPDF(s.config)
				localPath := fmt.Sprintf("file://%s", job.LocalPath)
				Log.Debug("convert html to pdf", localPath)
				pdf_path, err := htmlpdf.BuildFromLink(localPath)
				if err != nil {
					Log.Error(err)
					return
				}
				Log.Debug("convert html to pdf done", pdf_path)
				localFiles = append(localFiles, pdf_path)
			}(item, &wg)

			continue
		}
	}

	Log.Debug("wait for all download job done")
	wg.Wait()

	defer func() {
		for _, file := range localFiles {
			os.Remove(file)
		}
	}()

	Log.Debug("combine pdf")

	var combine_path string

	if len(localFiles) == 1 {
		combine_path = localFiles[0]
	} else {
		combineFile, err := os.CreateTemp("", "*.pdf")
		if err != nil {
			http.Error(writer, err.Error(), 500)
			return
		}
		combine_path = filepath.ToSlash(combineFile.Name())

		err = combineFile.Close()
		if err != nil {
			http.Error(writer, err.Error(), 500)
			return
		}

		err = CombinePDF(localFiles, combine_path)
		if err != nil {
			http.Error(writer, err.Error(), 500)
			return
		}
	}

	err := SetPDFMetaData(combine_path, &PDFMetaInfo{
		Author:   s.config.BuildMeta.Author,
		Creator:  s.config.BuildMeta.Creator,
		Keywords: s.config.BuildMeta.Keywords,
		Subject:  s.config.BuildMeta.Subject,
	})
	if err != nil {
		Log.Error(err)
	}

	download, err := os.Open(combine_path)
	if err != nil {
		http.Error(writer, err.Error(), 500)
		return
	}
	defer download.Close()

	Log.Debug("combine done")

	writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%d.pdf", time.Now().UnixNano()))
	writer.Header().Set("Content-Type", "application/pdf")
	_, err = io.Copy(writer, download)
	if err != nil {
		http.Error(writer, err.Error(), 500)
		return
	}
	defer time.AfterFunc(time.Second*10, func() {
		os.Remove(combine_path)
	})
}

func (s *HTTPService) COMBINE(writer http.ResponseWriter, request *http.Request) {
	if err := request.ParseForm(); err != nil {
		http.Error(writer, err.Error(), 500)
	}

	input_files := make([]string, 0)
	for key, values := range request.PostForm {
		if strings.EqualFold(key, "file") {
			input_files = append(input_files, values...)
		}
	}

	downloader := NewDownloader(input_files, s.config)
	queue := downloader.GetDownloadedFiles()

	localFiles := make([]string, 0)
	for item := range queue {
		if item.IsPDF() {
			localFiles = append(localFiles, item.LocalPath)
		}
	}
	defer func() {
		for _, file := range localFiles {
			os.Remove(file)
		}
	}()
	combineFile, err := os.CreateTemp("", "*.pdf")
	if err != nil {
		http.Error(writer, err.Error(), 500)
		return
	}
	combineFile.Close()
	combine_path := filepath.ToSlash(combineFile.Name())

	err = CombinePDF(localFiles, combine_path)
	if err != nil {
		http.Error(writer, err.Error(), 500)
		return
	}

	err = SetPDFMetaData(combine_path, &PDFMetaInfo{
		Author:   s.config.BuildMeta.Author,
		Creator:  s.config.BuildMeta.Creator,
		Keywords: s.config.BuildMeta.Keywords,
		Subject:  s.config.BuildMeta.Subject,
	})
	if err != nil {
		Log.Error(err)
	}

	download, err := os.Open(combine_path)
	if err != nil {
		http.Error(writer, err.Error(), 500)
		return
	}
	defer download.Close()

	writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%d.pdf", time.Now().UnixNano()))
	writer.Header().Set("Content-Type", "application/pdf")
	_, err = io.Copy(writer, download)
	if err != nil {
		http.Error(writer, err.Error(), 500)
		return
	}

	defer time.AfterFunc(time.Second*10, func() {
		os.Remove(combineFile.Name())
	})

}
