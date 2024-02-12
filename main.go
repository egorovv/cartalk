package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
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
		t, err := time.Parse("January 2, 2006", n.Data)
		if err == nil {
			info.Date = t.Format("2006-01-02")
		} else {
			info.Date = n.Data
		}
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
	name  string
	total int64
	len   int64
	src   io.Reader
}

func (p *Progress) Read(b []byte) (n int, err error) {
	n, err = p.src.Read(b)
	p.len += int64(n)
	fmt.Printf("\r%s %d :%d%%", p.name, p.len, 100*p.len/p.total)
	return
}

func downloadFile(filepath string, url string) (err error) {

	// Create the file
	part := filepath + ".part"
	out, err := os.Create(part)
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
		name:  filepath,
		total: resp.ContentLength,
		src:   resp.Body,
	}

	// Writer the body to file
	_, err = io.Copy(out, &p)
	if err != nil {
		return err
	}

	fmt.Printf("\n")
	os.Rename(part, filepath)
	return nil
}

func main() {

	args := struct {
		Podcast string
		Count   int
		Skip    int
	}{}

	flag.IntVar(&args.Count, "count", 20, "")
	flag.IntVar(&args.Skip, "skip", 0, "")
	flag.StringVar(&args.Podcast, "podcast", "510208", "")

	flag.Parse()

	if args.Podcast == "wait" {
		args.Podcast = "344098539"
	}

	url := "http://www.npr.org"

	pos := args.Skip
	a := []*Info{}
	for len(a) < args.Count {
		suffix := fmt.Sprintf("/get/%s/render/partial/next?start=%d", args.Podcast, pos+1)
		if pos == 0 {
			suffix = "/podcasts/" + args.Podcast
		}
		u := url + suffix
		x := Articles(u)
		pos = pos + len(x)
		if len(x) == 0 {
			fmt.Printf("out of episodes\n")
			break
		}

		a = append(a, x...)
	}

	a = a[0:args.Count]
	r, err := regexp.Compile("#[1234567890]+")
	if err != nil {
		fmt.Printf("%s\n", err)
	}

	fr, err := regexp.Compile("[/<>:\"\\|?*.,]")
	if err != nil {
		fmt.Printf("%s\n", err)
	}

	for _, i := range a {
		id := strings.TrimPrefix(i.Id, "res")
		episod := r.FindString(i.Title)
		i.Title = strings.Replace(i.Title, episod, "", -1)
		i.Title = strings.Trim(i.Title, ": ")
		title := strings.Replace(i.Title, "  ", " ", -1)
		title = strings.Replace(title, " ", "_", -1)
		title = fr.ReplaceAllString(title, "")

		if episod != "" {
			episod = strings.TrimPrefix(episod, "#")
		} else {
			episod = id
		}

		fn := i.Date + "-" + episod + "-" + i.Title + ".mp3"

		_, err = os.Stat(fn)
		if err != nil {
			downloadFile(fn, i.Url)
		}
	}
}
