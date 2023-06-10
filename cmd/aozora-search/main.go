package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"
	_ "github.com/mattn/go-sqlite3"
)

func showAuthors(db *sql.DB) error {
	rows, err := db.Query(`
        SELECT
            a.author_id,
            a.author
        FROM
            authors a
        ORDER BY
            CAST(a.author_id AS INTEGER)
    `)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var authorID, author string
		err = rows.Scan(&authorID, &author)
		if err != nil {
			return err
		}
		fmt.Printf("%s %s\n", authorID, author)
	}
	return nil
}

func showTitles(db *sql.DB, authorID string) error {
	rows, err := db.Query(`
        SELECT
            c.title_id,
            c.title
        FROM
            contents c
        WHERE
            c.author_id = ?
        ORDER BY
            CAST(c.title_id AS INTEGER)
    `, authorID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var titleID, title string
		err = rows.Scan(&titleID, &title)
		if err != nil {
			return err
		}
		fmt.Printf("% 5s %s\n", titleID, title)
	}
	return nil
}

func showContent(db *sql.DB, authorID string, titleID string) error {
	var content string
	err := db.QueryRow(`
        SELECT
            c.content
        FROM
            contents c
        WHERE
            c.author_id = ?
        AND c.title_id = ?
    `, authorID, titleID).Scan(&content)
	if err != nil {
		return err
	}
	fmt.Println(content)
	return nil
}

// SQLiteデータベースに対してフルテキスト検索を行い、その結果をコンソールに表示する
func queryContent(db *sql.DB, query string) error {
	// 分かち書きライブラリkagomeを使ってクエリ文字列を分かち書きします。
	// 分かち書きは、日本語のテキストを単語単位に分割するプロセスであり、テキスト検索や自然言語処理の前処理として一般的に行われます
	t, err := tokenizer.New(ipa.Dict(), tokenizer.OmitBosEos())
	if err != nil {
		return err
	}

	seg := t.Wakati(query)

	// authorsテーブルとcontentsテーブルを結合し、さらにcontents_ftsテーブルと結合してフルテキスト検索を行います。
	rows, err := db.Query(`
        SELECT
            a.author_id,
            a.author,
            c.title_id,
            c.title
        FROM
            contents c
        INNER JOIN authors a
            ON a.author_id = c.author_id
        INNER JOIN contents_fts f
            ON c.rowid = f.docid
            AND words MATCH ?
    `, strings.Join(seg, " "))
	if err != nil {
		return err
	}
	defer rows.Close()

	// クエリの結果が存在する限り（rows.Next()がtrueを返す限り）、その行のデータをスキャンして各変数に読み込み、結果を表示します。
	for rows.Next() {
		var authorID, author string
		var titleID, title string
		err = rows.Scan(&authorID, &author, &titleID, &title)
		if err != nil {
			return err
		}
		fmt.Printf("%s % 5s: %s (%s)\n", authorID, titleID, title, author)
	}
	return nil
}

const usage = `
Usage of ./aozora-search [sub-command] [...]:
  -d string
        database (default "database.sqlite")

Sub-commands:
    authors
    titles  [AuthorID]
    content [AuthorID] [TitleID]
    query   [Query]
`

func main() {
	var dsn string
	flag.StringVar(&dsn, "d", "database.sqlite", "database")
	flag.Usage = func() {
		fmt.Print(usage)
	}
	flag.Parse()

	// NArg()関数は、フラグ（オプション）以外のコマンドライン引数の数を返します。
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	switch flag.Arg(0) {
	case "authors":
		err = showAuthors(db)
	case "titles":
		// NArg()関数は、フラグ（オプション）以外のコマンドライン引数の数を返します。
		// titles  [AuthorID]
		if flag.NArg() != 2 {
			flag.Usage()
			os.Exit(2)
		}
		err = showTitles(db, flag.Arg(1))
	case "content":
		// NArg()関数は、フラグ（オプション）以外のコマンドライン引数の数を返します。
		// content [AuthorID] [TitleID]
		if flag.NArg() != 3 {
			flag.Usage()
			os.Exit(2)
		}
		err = showContent(db, flag.Arg(1), flag.Arg(2))
	case "query":
		// NArg()関数は、フラグ（オプション）以外のコマンドライン引数の数を返します。
		// query   [Query]
		if flag.NArg() != 2 {
			flag.Usage()
			os.Exit(2)
		}
		err = queryContent(db, flag.Arg(1))
	}

	if err != nil {
		log.Fatal(err)
	}
}
