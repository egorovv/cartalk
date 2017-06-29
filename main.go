package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func Articles(url string) (articles map[string]string) {

	articles = make(map[string]string)
	resp, err := http.Get(url)

	if err != nil {
		fmt.Printf("%s\n", err)
		return
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)

	if err != nil {
		fmt.Printf("%s\n", err)
		return
	}

	var f func(*html.Node, string)

	f = func(n *html.Node, id string) {
		if n.Type == html.ElementNode && n.DataAtom == atom.Article {
			for _, a := range n.Attr {
				if a.Key == "id" {
					id = a.Val
				}
			}
		} else if id != "" && n.Type == html.ElementNode && n.DataAtom == atom.A {
			for _, a := range n.Parent.Attr {
				if a.Key == "class" && a.Val == "audio-tool audio-tool-download" {
					for _, a := range n.Attr {
						if a.Key == "href" {
							articles[id] = a.Val
							return
						}
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c, id)
		}
	}

	f(doc, "")

	return
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

	// Writer the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

// "https://play.podtrac.com/npr-510208/npr.mc.tritondigital.com/NPR_510208/media/anon.npr-podcasts/podcast/510208/532389088/npr_532389088.mp3?orgId=1&amp;d=3291&amp;p=510208&amp;story=532389088&amp;t=podcast&amp;e=532389088&amp;siteplayer=true&amp;dl=1
func main() {

	url := "http://www.npr.org/podcasts/510208/car-talk"
	a := Articles(url)
	for {
		suffix := fmt.Sprintf("/partials?start=%d", len(a)+1)
		u := url + suffix
		fmt.Printf("%s\n", u)
		x := Articles(u)
		if len(x) == 0 {
			break
		}
		for k, v := range x {
			a[k] = v
		}
		if len(a) > 100 {
			break
		}
	}

	for x, url := range a {
		id := strings.TrimPrefix(x, "res")
		//url := fmt.Sprintf("https://play.podtrac.com/npr-510208/npr.mc.tritondigital.com/NPR_510208/media/anon.npr-podcasts/podcast/510208/%s/npr_%s.mp3?orgId=1&amp;d=3291&amp;p=510208&amp;story=%s&amp;t=podcast&amp;e=%s&amp;siteplayer=true&amp;dl=1",
		//	id, id, id, id)
		fmt.Printf("%s : %s\n", id, url)
		//downloadFile(id+".mp3", url)
	}
}
