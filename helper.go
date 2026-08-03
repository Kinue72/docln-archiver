package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/alexsergivan/transliterator"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"golang.org/x/text/width"
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
		for i := *rateLimitFlag; i > 0; i-- {
			fmt.Printf("\nVui lòng đợi %ds vì ăn rate limit (gay af)...", i)
			time.Sleep(time.Second)
			fmt.Printf("\033[1A\033[K")
		}
		fmt.Println()
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

func DecryptChapter(dataS, dataK, dataC string) (string, error) {
	var chunks []string
	if err := json.Unmarshal([]byte(dataC), &chunks); err != nil {
		return "", err
	}
	if len(chunks) == 0 {
		return "", nil
	}

	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i][:4] < chunks[j][:4]
	})

	var sb strings.Builder
	for _, chunk := range chunks {
		encoded := chunk[4:]
		var decoded []byte
		var err error

		switch dataS {
		case "xor_shuffle":
			decoded, err = xorDecrypt(encoded, dataK)
		case "base64_reverse":
			decoded, err = base64Decode(reverseString(encoded))
		default:
			decoded, err = base64Decode(encoded)
		}
		if err != nil {
			return "", err
		}
		sb.Write(decoded)
	}

	html := sb.String()
	html = reNote.ReplaceAllString(html, "")
	return html, nil
}

func xorDecrypt(encoded, key string) ([]byte, error) {
	raw, err := base64Decode(encoded)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(raw))
	for i := range raw {
		out[i] = raw[i] ^ key[i%len(key)]
	}
	return out, nil
}

func base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func IsStringASCII(input string) bool {
	for _, c := range input {
		if c >= utf8.RuneSelf {
			return false
		}
	}
	return true
}
