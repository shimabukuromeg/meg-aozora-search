package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractText(t *testing.T) {
	// httptest.NewServer(...) は、引数で与えられたハンドラを使って新しいテスト用のHTTPサーバを起動します。
	// http.FileServer(http.Dir(".")) は現在のディレクトリ（.）をルートとするファイルサーバを作成してる
	ts := httptest.NewServer(http.FileServer(http.Dir(".")))
	defer ts.Close()

	entry := Entry{
		AuthorID: "1",
		Author:   "テスト太郎",
		TitleID:  "1",
		Title:    "テスト",
		SiteURL:  "siteURL",
		ZipURL:   ts.URL + "/testdata/example.zip",
	}

	got, err := extractText(entry)
	if err != nil {
		t.Fatal(err)
		return
	}
	// テスト用のzipファイルには、あばばばばという文字列が含まれている
	want := "あばばばば"

	if want != got {
		t.Errorf("want %+v, but got %+v", want, got)
	}
}
