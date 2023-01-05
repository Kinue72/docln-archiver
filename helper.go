package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/alexsergivan/transliterator"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"golang.org/x/text/width"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var reUrl = regexp.MustCompile("url\\('(.*?)'\\)")

func ParseImageLink(selection *goquery.Selection) string {
	if attr, ok := selection.Attr("style"); ok && reUrl.MatchString(attr) {
		return reUrl.FindStringSubmatch(attr)[1]
	}
	return "https://docln.net/img/nocover.jpg"
}

func GetRequest(url, referer string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalln(err)
	}
	req.Header.Set("User-Agent", UserAgent)
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return resp, err
	}

	if resp.StatusCode == 429 {
		_ = resp.Body.Close()
		fmt.Println("Vui lòng đợi 60s vì ăn rate limit (gay af)...")
		time.Sleep(time.Second * 60)
		return GetRequest(url, referer)
	}

	return resp, err
}

var hasher = sha256.New()

func HashString(input string) string {
	hasher.Write([]byte(input))
	return hex.EncodeToString(hasher.Sum(nil))
}

var transformChain = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC, runes.If(runes.In(unicode.Latin), width.Fold, nil))
var translit = transliterator.NewTransliterator(nil)

// https://pubs.opengroup.org/onlinepubs/9699919799/basedefs/V1_chap03.html#tag_03_282
var reNotAllowChars = regexp.MustCompile("[^\\-_. \\w]")

func NormalizeString(input string) string {
	if input == "" {
		return ""
	}
	input = strings.TrimSpace(input)

	if IsStringASCII(input) {
		return input
	}

	output, _, _ := transform.String(transformChain, input)

	if output == "" {
		output = input
	}

	output = strings.ReplaceAll(output, "đ", "d")
	output = strings.ReplaceAll(output, "Đ", "D")

	if !IsStringASCII(output) {
		output = translit.Transliterate(output, "")
	}

	return reNotAllowChars.ReplaceAllString(output, "")
}

func IsStringASCII(input string) bool {
	for _, c := range input {
		if c >= utf8.RuneSelf {
			return false
		}
	}
	return true
}
