package main

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/bmaupin/go-epub"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Book struct {
	*epub.Epub

	Id      string
	BaseUrl string
}

var reNote = regexp.MustCompile("\\[note\\d*\\]")

func (b *Book) CrawlChapterBody(url string, retries int) string {
	if *retriesFlag > 0 && retries > *retriesFlag {
		return ""
	}

	resp, err := GetRequest(url, "")
	if err != nil {
		return b.CrawlChapterBody(url, retries+1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("status code: %d\n", resp.StatusCode)
		return ""
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return b.CrawlChapterBody(url, retries+1)
	}

	var sb strings.Builder

	title := doc.Find(".title-top > h4").Text()
	subTitle := strings.Split(doc.Find(".title-top > h6").Text(), " - ")[0] + " - Ngày đăng: " + doc.Find(".title-top > h6 > time").AttrOr("datetime", "Unknown")

	sb.WriteString(`<h2 class="chapter-header">`)
	sb.WriteString(title)
	sb.WriteString("</h2>")

	sb.WriteString(`<h4 class="chapter-header">`)
	sb.WriteString(subTitle)
	sb.WriteString("</h4>")

	sb.WriteString(`<hr class="header"/>`)

	sb.WriteString(`<div class="reading-content">`)

	doc.Find("#chapter-content > p").Each(func(_ int, selection *goquery.Selection) {
		if _, ok := selection.Attr("id"); !ok {
			return
		}

		//map img
		selection.Find("img").Each(func(_ int, selection *goquery.Selection) {
			src := selection.AttrOr("src", "")
			if src == "" {
				return
			}
			internal := b.CrawlImage(src, true, 0)
			if internal == "" {
				return
			}
			selection.SetAttr("class", "insert")
			selection.SetAttr("src", internal)
		})

		html, _ := selection.Html()

		notes := reNote.FindAllStringSubmatch(html, -1)
		html = reNote.ReplaceAllString(html, "")

		sb.WriteString("<p>")
		sb.WriteString(html)
		sb.WriteString("</p>")

		// poor man note :(
		if *noteFlag {
			for _, matches := range notes {
				note := strings.TrimLeft(matches[0], "[")
				note = strings.TrimRight(note, "]")

				text, _ := doc.Find("#" + note + " > .note-content_real").Html()
				if text == "" {
					continue
				}
				sb.WriteString("<p>(")
				sb.WriteString(note)
				sb.WriteString(": ")
				sb.WriteString(text)
				sb.WriteString(")</p>")
			}
		}
	})
	sb.WriteString("</div>")

	return sb.String()
}

var whitelistExts = []string{
	".png",
	".jpeg",
	".jpg",
	".webp",
}

func (b *Book) CrawlImage(path string, referer bool, retries int) string {
	if *retriesFlag > 0 && retries > *retriesFlag {
		return ""
	}

	ext := filepath.Ext(path)

	found := false
	for _, e := range whitelistExts {
		if strings.EqualFold(e, ext) {
			found = true
			break
		}
	}

	if !found {
		ext = ".jpg"
	}

	imgPath := filepath.Join("./tmp", b.Id, HashString(path)+ext)
	if _, err := os.Stat(imgPath); err == nil {
		ret, err := b.AddImage(imgPath, "")
		if err != nil {
			return b.CrawlImage(path, referer, retries+1)
		}
		return ret
	}

	ref := ""
	if referer {
		ref = b.BaseUrl + "/"
	}

	resp, err := GetRequest(path, ref)
	if err != nil {
		return b.CrawlImage(path, referer, retries+1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("status code: %d\n", resp.StatusCode)
		return ""
	}

	out, err := os.Create(imgPath)
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return b.CrawlImage(path, referer, retries+1)
	}

	ret, err := b.AddImage(imgPath, "")
	if err != nil {
		return b.CrawlImage(path, referer, retries+1)
	}
	return ret
}
