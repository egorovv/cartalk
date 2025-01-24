package main

import (
	"bufio"
	"encoding/json"
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
	Parts          []string
}

type PartInfo struct {
	AudioUrl string `json:"audioUrl"`
}

type EpisodeInfo struct {
	AudioData []PartInfo `json:"audioData"`
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

func walk2(n *html.Node, info *Info) (articles []*Info) {
	//if info == nil && n.Type == html.ElementNode && n.DataAtom == atom.Article {
	if info == nil && n.Type == html.ElementNode &&
		n.DataAtom == atom.Article && isClass(n, "program-show has-segments") {
		info = &Info{}
		info.Id = getAttr(n, "data-episode-id")
		info.Date = getAttr(n, "data-episode-date")
		articles = append(articles, info)
	} else if info != nil && n.Type == html.ElementNode && n.DataAtom == atom.B {
		data := getAttr(n, "data-play-all")
		if data != "" {
			x := EpisodeInfo{}
			json.Unmarshal([]byte(data), &x);
			if len(x.AudioData) > 0 {
				for _, y := range x.AudioData {
					info.Parts = append(info.Parts, y.AudioUrl)
				}
				return;
			}
		}
	} else if info != nil && n.Type == html.TextNode &&
		n.Parent.Parent != nil && isClass(n.Parent.Parent, "program-show__title") {
		info.Title = n.Data
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		articles = append(articles, walk2(c, info)...)
	}
	return
}

func Articles2(url string) (articles []*Info) {
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

	articles = walk2(doc, nil)

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

func mergeArticles(x []*Info, y []*Info) (z []*Info) {
	z = append(z, x...)
	out:
	for _, a := range y {
		for _, b := range x {
			if b.Date == a.Date {
				continue out
			}
		}
		z = append(z, a)
	}
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

func downloadFile(args Args, filepath string, url string) (err error) {
	if args.DryRun {
		fmt.Println(filepath)
		return nil
	}

	_, err = os.Stat(filepath)
	if err == nil {
		return nil
	}

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

type Args struct {
	Podcast string
	Urls    string
	Count   int
	Skip    int
	DryRun  bool
}

func main() {

	args := Args{}

	flag.IntVar(&args.Count, "count", 20, "")
	flag.IntVar(&args.Skip, "skip", 0, "")
	flag.StringVar(&args.Podcast, "podcast", "510208", "")
	flag.BoolVar(&args.DryRun, "dry-run", false, "dry run")
	flag.StringVar(&args.Urls, "urls", "", "")

	flag.Parse()

	if args.Podcast == "wait" {
		args.Podcast = "344098539"
	}

	url := "http://www.npr.org"

	a := []*Info{}

	if args.Urls != "" {
		in, err := os.Open(args.Urls);
		defer in.Close()

		if err != nil {
			fmt.Println(err)
			return;
		}
		scan := bufio.NewScanner(in)

		scan.Split(bufio.ScanLines)

		for scan.Scan() {
			x := Articles2(scan.Text())
			a = mergeArticles(a, x)
			if len(a) > args.Skip + args.Count {
				break
			}
		}

		a = a[args.Skip:args.Skip + args.Count]
	} else {
		pos := args.Skip
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
	}

	r, err := regexp.Compile("#[1234567890]+")
	if err != nil {
		fmt.Printf("%s\n", err)
	}

	fr, err := regexp.Compile("[/<>:\"\\|?*.,!()']")
	if err != nil {
		fmt.Printf("%s\n", err)
	}

	for idx, i := range a {
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

		no := idx + args.Skip

		if len(i.Parts) > 0 {
			for idx, part := range i.Parts {
				fn := fmt.Sprintf("%03d-%s-%s-%s-%d.mp3", no, i.Date, episod, title, idx + 1)
				downloadFile(args, fn, part)
			}
		} else {
			fn := fmt.Sprintf("%03d-%s-%s-%s.mp3", no, i.Date, episod, title)
			downloadFile(args, fn, i.Url)
		}
	}
}
