package lib

import (
	"context"
	_ "embed"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

//go:embed motorsPDFjsPatch.js
var MOTORS_PDF_JS_PATCH string

type TaskResult struct {
	File  string
	Err   error
	Index int
}

// type Task struct {
// 	taskJob   chan *TaskResult
// 	taskCount int
// }

type PDFOption struct {
	page.PrintToPDFParams

	patchMotors bool
}

type HTMLPDF struct {
	config    *Config
	pdfOption *PDFOption
	jobQueue  chan bool
}

// 适配cwinlis
type PagerMargin struct {
	Top    float64 `json:"top"`
	Right  float64 `json:"right"`
	Bottom float64 `json:"bottom"`
	Left   float64 `json:"left"`
}

type PapperOption struct {
	Scale             float64      `json:"scale"`
	PaperFormat       string       `json:"paperFormat"`
	Width             float64      `json:"width"`
	Height            float64      `json:"height"`
	Landscape         bool         `json:"landscape"`
	PreferCSSPageSize bool         `json:"preferCSSPageSize"`
	PagerMargin       *PagerMargin `json:"pagerMargin"`
	PrintBackground   bool         `json:"printBackground"`
}

type PaperSize int

const (
	A0 PaperSize = iota
	A1
	A2
	A3
	A4
	A5
	A6
	LETTER
	LEGAL
	TABLOID
	LEDGER
)

func (ps PaperSize) String() string {
	return []string{"A0", "A1", "A2", "A3", "A4", "A5", "A6", "LETTER", "LEGAL", "TABLOID", "LEDGER"}[ps]
}

func (ps PaperSize) getWidth() float64 {
	switch ps {
	case A0:
		return 33.11
	case A1:
		return 23.39
	case A2:
		return 16.54
	case A3:
		return 11.69
	case A4:
		return 8.27
	case A5:
		return 5.83
	case A6:
		return 4.13
	case LETTER:
		return 8.5
	case LEGAL:
		return 8.5
	case TABLOID:
		return 11
	case LEDGER:
		return 17
	}
	return 0
}

func (ps PaperSize) getHeight() float64 {
	switch ps {
	case A0:
		return 46.81
	case A1:
		return 33.11
	case A2:
		return 23.39
	case A3:
		return 16.54
	case A4:
		return 11.69
	case A5:
		return 8.27
	case A6:
		return 5.83
	case LETTER:
		return 11
	case LEGAL:
		return 14
	case TABLOID:
		return 17
	case LEDGER:
		return 11
	}
	return 0
}

func NewHTMLPDF(conf *Config) *HTMLPDF {
	return &HTMLPDF{
		config:   conf,
		jobQueue: make(chan bool, conf.Worker),
		pdfOption: &PDFOption{
			PrintToPDFParams: page.PrintToPDFParams{
				PaperWidth:        8.27,  //A4
				PaperHeight:       11.69, //A4
				MarginTop:         0,
				MarginRight:       0,
				MarginBottom:      0,
				MarginLeft:        0,
				Scale:             1,
				Landscape:         false,
				PrintBackground:   true,
				PreferCSSPageSize: false,
			},
			patchMotors: false,
		},
	}
}

func (pdf *HTMLPDF) WithParams(params *page.PrintToPDFParams) *HTMLPDF {
	pdf.pdfOption = &PDFOption{
		PrintToPDFParams: *params,
		patchMotors:      pdf.pdfOption.patchMotors,
	}
	return pdf
}

func (pdf *HTMLPDF) WithParamsRun(url string, params *page.PrintToPDFParams) (string, error) {
	pdf.jobQueue <- true
	defer func() {
		<-pdf.jobQueue
	}()

	return pdf.WithParams(params).run(url)
}

func (pdf *HTMLPDF) run(url string) (string, error) {
	// 將 PrintToPDFParams 轉換為 CSS @page 樣式
	if pdf.pdfOption.Scale == 0 {
		pdf.pdfOption.Scale = 1
	}
	customCSS := ""
	if pdf.pdfOption.Landscape {
		customCSS = fmt.Sprintf(`
			@page {
				size: %.2fin %.2fin;
				margin: %.2fin %.2fin %.2fin %.2fin;
			}
		`, pdf.pdfOption.PaperHeight, pdf.pdfOption.PaperWidth, pdf.pdfOption.MarginTop, pdf.pdfOption.MarginRight, pdf.pdfOption.MarginBottom, pdf.pdfOption.MarginLeft)
	} else {
		customCSS = fmt.Sprintf(`
		@page {
			size: %.2fin %.2fin;
			margin: %.2fin %.2fin %.2fin %.2fin;
		}
	`, pdf.pdfOption.PaperWidth, pdf.pdfOption.PaperHeight, pdf.pdfOption.MarginTop, pdf.pdfOption.MarginRight, pdf.pdfOption.MarginBottom, pdf.pdfOption.MarginLeft)
	}

	PreferCSSPageSize := false
	if pdf.pdfOption.PreferCSSPageSize || pdf.pdfOption.patchMotors {
		PreferCSSPageSize = true
	}

	dpi := 150.0
	PaperHeight := pdf.pdfOption.PaperHeight
	PaperWidth := pdf.pdfOption.PaperWidth
	if pdf.pdfOption.Landscape {
		PaperHeight = pdf.pdfOption.PaperWidth
		PaperWidth = pdf.pdfOption.PaperHeight
	}
	// 转换为视口尺寸（以像素为单位）
	viewportWidth := int(PaperWidth * dpi)
	viewportHeight := int(PaperHeight * dpi)

	Log.Debugf("PaperHeight: %f, PaperWidth: %f, dpi: %f, viewportWidth: %d, viewportHeight: %d", PaperHeight, PaperWidth, dpi, viewportWidth, viewportHeight)

	// 自定義 Chrome 路徑
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(pdf.config.ChromePath),
		chromedp.DisableGPU,
		chromedp.Flag("disable-web-security", true),
		chromedp.WindowSize(viewportWidth, viewportHeight+50),
	)

	// logLevel, ok := os.LookupEnv("LOG_LEVEL")
	// if ok && logLevel == "DEBUG" {
	// 	opts = append(opts, chromedp.Flag("headless", false))
	// }

	defaultCtx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(pdf.config.Timeout))
	defer cancel()

	// 創建上下文
	ctx, cancel := chromedp.NewExecAllocator(defaultCtx, opts...)
	defer cancel()

	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()

	// 创建一个事件监听器来监听页面加载完成事件
	loadEventFired := make(chan struct{})
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev.(type) {
		case *page.EventLoadEventFired:
			close(loadEventFired)
		}

	})

	var buf []byte

	// 适配cwinlis
	selector := "body"
	if pdf.config.PatchCwinlis {
		selector = ".previewer-ready-to-pdf"
	}

	err := chromedp.Run(ctx, chromedp.Tasks{
		chromedp.Navigate(url),
		chromedp.WaitReady(selector),
		chromedp.ActionFunc(func(ctx context.Context) error {
			Log.Debug("chromedp js patch")
			if pdf.pdfOption.patchMotors {
				err := chromedp.Evaluate(string(MOTORS_PDF_JS_PATCH), nil).Do(ctx)
				if err != nil {
					Log.Error(err)
					return err
				}

				return chromedp.WaitReady("span[data-scrip-done=true]").Do(ctx)
			}
			return nil
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			Log.Debug("chromedp inject css")
			if !pdf.pdfOption.patchMotors && PreferCSSPageSize && len(customCSS) > 0 {
				return chromedp.Evaluate(fmt.Sprintf(`(function() {
						var style = document.createElement('style');
						style.type = 'text/css';
						style.innerHTML = %q;
						document.head.appendChild(style);
					})()`, customCSS), nil).Do(ctx)
			}

			return nil
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			select {
			case <-loadEventFired:
				Log.Debug("chromedp load event fired")
				var err error

				pdf.pdfOption.PreferCSSPageSize = PreferCSSPageSize

				buf, _, err = pdf.pdfOption.Do(ctx)
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
		}),
	})
	if err != nil {
		Log.Error(err)
		return "", err
	}
	defer page.Close()

	tmpFile, err := os.CreateTemp("", "*.pdf")
	if err != nil {
		Log.Error(err)
		return "", err
	}
	defer tmpFile.Close()
	// 保存 PDF 文件
	if _, err := tmpFile.Write(buf); err != nil {
		Log.Error(err)
		return "", err
	}

	return filepath.ToSlash(tmpFile.Name()), nil
}

func (pdf *HTMLPDF) BuildFromLink(link string) (local_pdf string, err error) {
	pdf_name, err := pdf.run(link)
	if err != nil {
		return "", err
	}
	return pdf_name, nil
}

func (pdf *HTMLPDF) BuildFromSource(html []byte) (local_pdf string, err error) {

	tmpFile, err := os.CreateTemp("", "*.html")
	if err != nil {
		Log.Error(err)
		return "", err
	}
	defer tmpFile.Close()
	// 保存 PDF 文件
	if _, err := tmpFile.Write(html); err != nil {
		Log.Error(err)
		return "", err
	}

	pdf_name, err := pdf.run(fmt.Sprintf("file://%s", tmpFile.Name()))
	if err != nil {
		return "", err
	}

	return pdf_name, nil
}

func (pdf *HTMLPDF) Combine(files []string) (dest_pdf_path string, err error) {
	tmpFile, err := ioutil.TempFile("", "*.html")
	if err != nil {
		Log.Error(err)
		return "", err
	}
	tmpFile.Close()

	pdf_name := filepath.ToSlash(tmpFile.Name())
	err = CombinePDF(files, pdf_name)
	if err != nil {
		return pdf_name, err
	}
	return pdf_name, nil
}
