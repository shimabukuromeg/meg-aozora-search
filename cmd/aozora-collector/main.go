package main

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"
	"golang.org/x/text/encoding/japanese"

	_ "github.com/mattn/go-sqlite3"
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
	// 指定したURLのHTMLドキュメントを取得します
	doc, err := goquery.NewDocument(siteURL)
	if err != nil {
		return "", ""
	}
	// 著者の名前をHTMLドキュメントから探し出す
	author := doc.Find("table[summary=作家データ] tr:nth-child(2) td:nth-child(2)").First().Text()
	zipURL := ""
	doc.Find("table.download a").Each(func(n int, elem *goquery.Selection) {
		href := elem.AttrOr("href", "")
		// href属性（リンク先）が.zipで終わるものがあれば、それをzipURLに保存する
		if strings.HasSuffix(href, ".zip") && !strings.HasSuffix(href, "ttz.zip") {
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
	// URLが特定の形式（".*/cards/[0-9]+/card[0-9]+.html"）に一致するかどうかを確認するため	の正規表現
	pat := regexp.MustCompile(`.*/cards/([0-9]+)/card([0-9]+).html$`)
	entries := []Entry{}
	// doc.Findはgoqueryのメソッドで、DOM内のすべての<ol>（順序付けられたリスト）要素の中の<li>（リスト項目）の中の<a>（アンカー）要素を探し、それぞれに対して処理を行う
	doc.Find("ol li a").Each(func(n int, elem *goquery.Selection) {

		// 各<a>要素のhref属性（リンク先URL）を取得し、先程の正規表現に一致するかを確認します。
		// 一致する場合、tokenは一致したグループ（この場合は2つの数値）を含む配列になります。
		token := pat.FindStringSubmatch(elem.AttrOr("href", ""))
		if len(token) != 3 {
			return
		}
		// titleは<a>要素のテキスト内容（リンクテキスト）を表します。
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

func extractText(entry Entry) (string, error) {
	// URLからHTTP GETリクエストを送信し、レスポンスを取得する
	resp, err := http.Get(entry.ZipURL)
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
	jsonData, err := json.Marshal(entry)
	if err != nil {
		fmt.Println("JSONマーシャリングに失敗しました：", err)
		return "", nil
	}
	return "", errors.New("contents not found " + string(jsonData))
}

func setupDB(dsn string) (*sql.DB, error) {
	// sql.Open("sqlite3", dsn)を用いて、指定されたデータソース（dsn）でSQLite3データベースに接続
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	// テーブルを作成するQuery
	queries := []string{
		`CREATE TABLE IF NOT EXISTS authors(author_id TEXT, author TEXT, PRIMARY KEY (author_id))`,
		`CREATE TABLE IF NOT EXISTS contents(author_id TEXT, title_id TEXT, title TEXT, content TEXT, PRIMARY KEY (author_id, title_id))`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS contents_fts USING fts4(words)`,
	}
	for _, query := range queries {
		// db.Exec(query)を用いて、Queryを実行
		_, err = db.Exec(query)
		if err != nil {
			return nil, err
		}
	}
	return db, nil
}

func addEntry(db *sql.DB, entry *Entry, content string) error {
	// [INSERT 1] エントリーの作者IDと作者が挿入されます。
	_, err := db.Exec(`
        REPLACE INTO authors(author_id, author) values(?, ?)
    `,
		entry.AuthorID,
		entry.Author,
	)
	if err != nil {
		return err
	}

	// [INSERT 2] エントリーの作者ID、タイトルID、タイトル、そしてコンテンツが挿入されます。
	res, err := db.Exec(`
        REPLACE INTO contents(author_id, title_id, title, content) values(?, ?, ?, ?)
    `,
		entry.AuthorID,
		entry.TitleID,
		entry.Title,
		content,
	)
	if err != nil {
		return err
	}

	// 上記で挿入した行のIDを取得します。これはcontents_ftsテーブルへの挿入に使用されます。
	docID, err := res.LastInsertId()
	if err != nil {
		return err
	}

	// 形態素解析器（トークナイザー）を作成します。このトークナイザーは日本語の文章を単語に分割するために使用されます
	t, err := tokenizer.New(ipa.Dict(), tokenizer.OmitBosEos())
	if err != nil {
		return err
	}

	// コンテンツを単語に分割します。この結果はsegとして取得されます。
	seg := t.Wakati(content)

	// [INSERT 3] テーブルに文書IDと分割された単語のリストが挿入（あるいは既存の同一IDの行があれば置換）されます。
	// 分割された単語のリストはスペースで連結され、一つの文字列として格納されます。
	_, err = db.Exec(`
        REPLACE INTO contents_fts(docid, words) values(?, ?)
    `,
		docID,
		strings.Join(seg, " "),
	)
	if err != nil {
		return err
	}
	return nil
}

func main() {
	db, err := setupDB("database.sqlite")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	listURL := "https://www.aozora.gr.jp/index_pages/person879.html"
	entries, err := findEntries(listURL)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("found %d entries", len(entries))
	for _, entry := range entries {
		content, err := extractText(entry)
		if err != nil {
			log.Fatal(err)
			continue
		}
		// `&` をつけると、値のアドレスを取得できる。
		err = addEntry(db, &entry, content)
		if err != nil {
			log.Println(err)
			continue
		}
		time.Sleep(time.Second * 1)
	}
}
