package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/bmaupin/go-epub"
)

const UserAgent = "Mozilla/5.0 (Windows NT 10.0; rv:108.0) Gecko/20100101 Firefox/108.0"

var (
	retriesFlag = flag.Int("retries", 10, "retries count")
	timeoutFlag = flag.Duration("timeout", time.Second*10, "http timeout")
	noteFlag    = flag.Bool("note", true, "enable note")
	linkFlag    = flag.String("link", "", "link to archive")
)

//go:embed epub.css
var cssFile string

func init() {
	flag.Parse()
	if *linkFlag == "" {
		flag.Usage()
		os.Exit(1)
	}
	_ = os.Mkdir("./tmp", 0700)
	_ = os.Mkdir("./output/", 0700)
	http.DefaultClient.Timeout = *timeoutFlag

	if _, err := os.Stat("./epub.css"); err != nil {
		f, err := os.OpenFile("./epub.css", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			log.Fatalln(err)
		}

		defer f.Close()
		_, _ = f.WriteString(cssFile)
	}
}

func main() {
	resp, err := GetRequest(*linkFlag, "")
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Fatalf("status code: %d %s", resp.StatusCode, resp.Status)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	var series Series

	series.Title = strings.TrimSpace(doc.Find(".series-name > a").Text())
	series.Translator = strings.TrimSpace(doc.Find(".series-owner_name > a").Text())
	series.Group = strings.TrimSpace(doc.Find(".fantrans-value > a").Text())
	series.Cover = ParseImageLink(doc.Find(".series-cover > .a6-ratio > div"))
	series.Description = doc.Find(".summary-content > p").Map(func(_ int, selection *goquery.Selection) string {
		return strings.TrimSpace(selection.Text())
	})

	if attr, ok := doc.Find(".sharing-item").Attr("@click.prevent"); ok {
		attr = strings.TrimSpace(attr)
		attr = strings.TrimLeft(attr, "window.navigator.clipboard.writeText('")
		attr = strings.TrimRight(attr, "')")
		split := strings.Split(attr, "/truyen/")

		series.BaseUrl = split[0]
		series.Id = split[1]
	}

	doc.Find(".info-item").Each(func(_ int, selection *goquery.Selection) {
		name := strings.TrimSpace(selection.Find(".info-name").Text())
		value := strings.TrimSpace(selection.Find(".info-value > a").Text())
		switch name {
		case "Tác giả:":
			series.Author = value
			break
		case "Họa sĩ:":
			series.Artist = value
			break
		case "Tình trạng:":
			series.Status = value
			break
		}
	})

	doc.Find(".volume-list").Each(func(_ int, selection *goquery.Selection) {
		var vol Volume
		vol.Title = strings.TrimSpace(selection.Find(".sect-title").Text())
		vol.Cover = ParseImageLink(selection.Find(".volume-cover > a > .a6-ratio > div"))

		selection.Find(".list-chapters > li").Each(func(_ int, selection *goquery.Selection) {
			a := selection.Find(".chapter-name > a")
			vol.Chapters = append(vol.Chapters, Chapter{
				Title: strings.TrimSpace(a.Text()),
				Url:   series.BaseUrl + a.AttrOr("href", ""),
			})
		})

		series.Volumes = append(series.Volumes, vol)
	})

	if len(series.Volumes) == 0 {
		panic("Sai định dạng link hoặc truyện không tồn tại?")
	}

	fmt.Printf("Tên: %s (%s) - %s\n", series.Title, series.Id, series.Status)
	fmt.Printf("Tác giả: %s - Minh hoạ: %s\n", series.Author, series.Artist)
	fmt.Printf("Dịch giả: %s - Nhóm dịch: %s\n", series.Translator, series.Group)
	fmt.Printf("Số volume: %d tập\n", len(series.Volumes))

	outputPath := filepath.Join("./output", series.Id)

	_ = os.Mkdir(outputPath, 0700)
	_ = os.Mkdir(filepath.Join("./tmp", series.Id), 0700)

	for i, vol := range series.Volumes {
		fmt.Println("-----------------------------------")
		fmt.Printf("Đang crawl volume %d: %s\n", i+1, vol.Title)
		book := Book{
			Epub:    epub.NewEpub(fmt.Sprintf("%s: Volume %d", vol.Title, i+1)),
			Id:      series.Id,
			BaseUrl: series.BaseUrl,
		}

		book.SetTitle(series.Title + " - " + vol.Title)
		book.SetAuthor(series.Author)
		book.SetLang("vi")
		book.SetIdentifier(*linkFlag)

		book.SetDescription(strings.Join(series.Description, "\n"))

		epubCss, _ := book.AddCSS("./epub.css", "")
		book.SetCover(book.CrawlImage(series.Cover, false, 0), epubCss)

		for j, chap := range vol.Chapters {
			fmt.Printf("Đang crawl chapter: %s\n", chap.Title)
			body := book.CrawlChapterBody(chap.Url, 0)

			if body == "" {
				continue
			}

			// too lazy to normalize title
			_, err = book.AddSection(body, chap.Title, fmt.Sprintf("chapter%d.xhtml", j), epubCss)
			if err != nil {
				log.Fatalln(err)
			}
		}

		//header
		{
			var sb strings.Builder

			sb.WriteString(`<h2 class="chapter-header">`)
			sb.WriteString(series.Title)
			sb.WriteString(`</h2> <hr class="header"/>`)

			sb.WriteString("<p>" + fmt.Sprintf("Tác giả: %s - Minh hoạ: %s", series.Author, series.Artist) + "</p>")
			sb.WriteString("<p>" + fmt.Sprintf("Dịch giả: %s - Nhóm dịch: %s", series.Translator, series.Group) + "</p>")
			sb.WriteString("<p>Link gốc: " + `<a href="` + *linkFlag + `">` + *linkFlag + "</a></p>")

			// too lazy to normalize title
			_, err = book.AddSection(sb.String(), "Footer", "footer.xhtml", epubCss)
			if err != nil {
				log.Fatalln(err)
			}
		}

		err = book.Write(filepath.Join(outputPath, fmt.Sprintf("%s %s - %d.epub", series.Id, NormalizeString(vol.Title), i+1)))
		if err != nil {
			log.Fatalln(err)
		}
	}
}
