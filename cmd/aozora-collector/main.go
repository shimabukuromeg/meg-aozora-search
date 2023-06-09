package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/encoding/japanese"
)

type Entry struct {
	AuthorID string
	Author   string
	TitleID  string
	Title    string
	SiteURL  string
	ZipURL   string
}

// 作者とZIPファイルのURLを取得する
func findAuthorAndZIP(siteURL string) (string, string) {
	// log.Println("query", siteURL)
	doc, err := goquery.NewDocument(siteURL)
	if err != nil {
		return "", ""
	}
	author := doc.Find("table[summary=作家データ] tr:nth-child(2) td:nth-child(2)").First().Text()
	zipURL := ""
	doc.Find("table.download a").Each(func(n int, elem *goquery.Selection) {
		href := elem.AttrOr("href", "")
		if strings.HasSuffix(href, ".zip") {
			zipURL = href
		}
	})

	if zipURL == "" {
		return author, ""
	}

	// zip url が最初から絶対パスで指定されている場合はそのまま返す
	if strings.HasPrefix(zipURL, "http://") || strings.HasPrefix(zipURL, "https://") {
		return author, zipURL
	}

	u, err := url.Parse(siteURL)
	if err != nil {
		return author, ""
	}
	u.Path = path.Join(path.Dir(u.Path), zipURL)
	return author, u.String()
}

var pageURLFormat = "https://www.aozora.gr.jp/cards/%s/card%s.html"

func findEntries(siteURL string) ([]Entry, error) {
	// goqueryでURLからDOMオブジェクトを取得する
	doc, err := goquery.NewDocument(siteURL)
	if err != nil {
		return nil, err
	}
	pat := regexp.MustCompile(`.*/cards/([0-9]+)/card([0-9]+).html$`)
	entries := []Entry{}
	doc.Find("ol li a").Each(func(n int, elem *goquery.Selection) {
		token := pat.FindStringSubmatch(elem.AttrOr("href", ""))
		if len(token) != 3 {
			return
		}
		title := elem.Text()
		pageURL := fmt.Sprintf(pageURLFormat, token[1], token[2])
		author, zipURL := findAuthorAndZIP(pageURL)
		if zipURL != "" {
			entries = append(entries, Entry{
				AuthorID: token[1],
				Author:   author,
				TitleID:  token[2],
				Title:    title,
				SiteURL:  siteURL,
				ZipURL:   zipURL,
			})
		}
	})

	return entries, nil
}

func extractText(zipURL string) (string, error) {
	// URLからHTTP GETリクエストを送信し、レスポンスを取得する
	resp, err := http.Get(zipURL)
	if err != nil {
		return "", err
	}
	// deferの後に続く関数の実行を、現在の関数が終了する直前まで遅らせる
	// 現在の関数が終了する直前に、resp.Body.Close()を実行して、HTTPレスポンスのボディをクローズする
	// これを怠ると、ネットワークリソースが不適切に保持され続け、リソースリークの原因となる可能性がある
	defer resp.Body.Close()

	// HTTPレスポンスのボディ部分を全て読み込みます
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	// 読み込んだZIPファイルの内容をメモリ上に展開
	r, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	// 展開した各ファイルをループで処理します。このループの中で.txtファイルを探しています
	for _, file := range r.File {
		if path.Ext(file.Name) == ".txt" {
			// .txtファイルを開く
			f, err := file.Open()
			if err != nil {
				return "", err
			}
			// 全て読み込みます
			b, err := ioutil.ReadAll(f)
			f.Close()
			if err != nil {
				return "", err
			}
			// 読み込んだ.txtファイルの内容をShift-JISからUTF-8に変換します
			b, err = japanese.ShiftJIS.NewDecoder().Bytes(b)
			if err != nil {
				return "", err
			}
			// 変換したテキスト内容を文字列として返します
			return string(b), nil
		}
	}
	return "", errors.New("contents not found")
}

func main() {
	listURL := "https://www.aozora.gr.jp/index_pages/person879.html"
	entries, err := findEntries(listURL)
	if err != nil {
		log.Fatal(err)
	}

	for _, entry := range entries {
		content, err := extractText(entry.ZipURL)
		if err != nil {
			log.Fatal(err)
			continue
		}
		fmt.Println(entry.SiteURL)
		fmt.Println(content)
	}
}
