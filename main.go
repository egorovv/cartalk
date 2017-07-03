package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	id3 "github.com/mikkyang/id3-go"
	//id3 "github.com/bogem/id3v2"
)

func getAttr(n *html.Node, attr string) string {
	for _, a := range n.Attr {
		if a.Key == attr {
			return a.Val
		}
	}
	return ""
}

func isClass(n *html.Node, class string) bool {
	return getAttr(n, "class") == class
}

type Info struct {
	Id, Title, Url string
	Teaser         string
	Date           string
}

func walk(n *html.Node, info *Info) (articles []*Info) {
	//if info == nil && n.Type == html.ElementNode && n.DataAtom == atom.Article {
	if info == nil && n.Type == html.ElementNode && isClass(n, "item-info") {
		//id := getAttr(n, "id")
		//if id != "" {
		//	info = &Info{
		//		Id: id,
		//	}
		info = &Info{}
		articles = append(articles, info)
		//}
	} else if info != nil && isClass(n.Parent, "date") && isClass(n.Parent.Parent.Parent, "episode-date") {
		info.Date = n.Data
	} else if info != nil && isClass(n.Parent, "teaser") {
		info.Teaser = n.Data
	} else if info != nil && n.Type == html.ElementNode && n.DataAtom == atom.Article {
		info.Id = getAttr(n, "id")
	} else if info != nil && n.Type == html.ElementNode && n.DataAtom == atom.A {
		if isClass(n.Parent, "audio-tool audio-tool-download") {
			url := getAttr(n, "href")
			if url != "" {
				info.Url = url
				return
			}
		}
	} else if info != nil && n.Type == html.TextNode && isClass(n.Parent, "audio-module-title") {
		info.Title = n.Data
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		articles = append(articles, walk(c, info)...)
	}
	return
}

func Articles(url string) (articles []*Info) {
	fmt.Printf("%s\n", url)
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

	articles = walk(doc, nil)

	return
}

type Progress struct {
	total int64
	len   int64
	src   io.Reader
}

func (p *Progress) Read(b []byte) (n int, err error) {
	n, err = p.src.Read(b)
	p.len += int64(n)
	fmt.Printf("\r%d :%d%%", p.len, 100*p.len/p.total)
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

	p := Progress{
		total: resp.ContentLength,
		src:   resp.Body,
	}

	// Writer the body to file
	_, err = io.Copy(out, &p)
	if err != nil {
		return err
	}
	fmt.Printf("\n")
	return nil
}

func main() {

	args := struct {
		Podcast string
		Count   int
	}{}

	flag.IntVar(&args.Count, "count", 20, "")
	flag.StringVar(&args.Podcast, "podcast", "510208/car-talk", "")

	flag.Parse()

	url := "http://www.npr.org/podcasts/" + args.Podcast

	a := Articles(url)
	for len(a) < args.Count {
		suffix := fmt.Sprintf("/partials?start=%d", len(a)+1)
		u := url + suffix
		x := Articles(u)
		if len(x) == 0 {
			break
		}
		a = append(a, x...)
	}

	a = a[0:args.Count]
	r, err := regexp.Compile("#[1234567890]+")
	if err != nil {
		fmt.Printf("%s\n", err)
	}

	for _, i := range a {
		id := strings.TrimPrefix(i.Id, "res")
		episod := r.FindString(i.Title)

		fmt.Printf("%s : %s : %s : %s\n", episod, id, i.Title, i.Date)
		if episod != "" {
			episod = strings.TrimPrefix(episod, "#")
		} else {
			episod = id
		}

		fn := episod + ".mp3"
		downloadFile(fn, i.Url)

		tag, err := id3.Open(fn)
		if err != nil {
			fmt.Printf("%s\n", err)
			continue
		}
		defer tag.Close()
		tag.SetArtist("Car Talk")
		tag.SetTitle(i.Title)
	}
}
