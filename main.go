package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/anaskhan96/soup"
	"github.com/avast/retry-go"
)

const (
	src = "https://www.tingclass.net/list-7751-%d.html"
)

type Show struct {
	siteURL   string
	Title     string
	SeqNumber string
}

func main() {
	shows := make(chan *Show, 1000)
	go func() {
		for pageNumber := 1; pageNumber <= 24; pageNumber++ {
			listURL := fmt.Sprintf(src, pageNumber)
			err := retry.Do(func() error {
				resp, err := soup.Get(listURL)
				if err != nil {
					return err
				}
				doc := soup.HTMLParse(resp)
				children := doc.FindAll("li", "class", "clearfix")

				/**
				<li class="clearfix">
				   <em class="fr">浏览：676</em>
				   <span class="fl class_num">第461篇:</span>
					<a class="ell" href="https://www.tingclass.net/show-7751-334158-1.html" target="_blank" title="Eating in Hongkong">Eating in Hongkong</a>
				</li>
				*/

				for i := range children {
					child := children[i]
					span := child.Find("span", "class", "class_num")
					a := child.Find("a", "class", "ell")
					seqNumber := strings.TrimLeft(strings.TrimRight(span.Text(), "篇:"), "第")
					shows <- &Show{
						siteURL:   a.Attrs()["href"],
						SeqNumber: seqNumber,
						Title:     a.Attrs()["title"],
					}
				}
				return nil
			})

			if err != nil {
				println(fmt.Sprintf("url=%s,err=%v", listURL, err))
				break
			}
		}
		close(shows)
	}()

	wg := &sync.WaitGroup{}
	for show := range shows {
		wg.Add(1)

		println(show.siteURL)
		go func(s *Show) {
			defer wg.Done()
			err := retry.Do(func() error {
				resp, err := soup.Get(s.siteURL)
				if err != nil {
					return err
				}
				doc := soup.HTMLParse(resp)
				children := doc.FindAll("div", "id", "mp3")
				println(len(children))
				for i := range children {
					downloadURL := children[i].Text()
					prefix, postfix := urlFileName(downloadURL)
					fileName := buildFilename(fmt.Sprintf("%s#%s %v.%s", prefix, s.SeqNumber, s.Title, postfix))
					println(fmt.Sprintf("%v,url=%s", fileName, downloadURL))
					if err = downloadFile(fileName, downloadURL); err != nil {
						println(err)
						break
					}
				}
				return nil
			})
			if err != nil {
				println(fmt.Sprintf("url=%v,err:=%v", s.siteURL, err))
			}
		}(show)
	}
	wg.Wait()
}

func buildFilename(s string) string {
	return strings.ReplaceAll(
		strings.ReplaceAll(
			strings.ReplaceAll(
				strings.ReplaceAll(
					strip(s), "  ", "",
				), " \" ", "\""),
			"  ", ""),
		"？", "")
}

func strip(s string) string {
	var result strings.Builder
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b != '"' {
			result.WriteByte(b)
		}
	}
	return result.String()
}
func downloadFile(filepath string, url string) (err error) {
	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check server response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Writer the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func urlFileName(url string) (string, string) {
	r, _ := http.NewRequest("GET", url, nil)
	parts := strings.Split(path.Base(r.URL.Path), ".")

	return "20" + strings.Join(parts[0:len(parts)-1], ""), parts[len(parts)-1]
}
