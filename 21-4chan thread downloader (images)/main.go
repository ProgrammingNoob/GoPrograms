package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
)

type Image struct {
	Name string
	Url  string
}

func (this *Image) Download(directory string) error {
	res, err := http.Get(this.Url)
	if err != nil {
		return err
	}

	defer res.Body.Close()
	b, err2 := ioutil.ReadAll(res.Body)
	if err2 != nil {
		return err2
	}

	return ioutil.WriteFile(path.Join(directory, this.Name), b, 0644)
}

var (
	threadUrlRegex = regexp.MustCompile(`^(https?://)?boards.4chan.org/(?P<board>[a-z0-9]{1,4})/res/(?P<no>\d+)(#q\d+)?$`)
)

func jsonThreadUrl(threadUrl string, pBoard, pNo *string) string {
	submatches := threadUrlRegex.FindStringSubmatch(threadUrl)
	if submatches == nil {
		panic("Invalid thread URL")
	}
	var board, no string
	for i, subexpName := range threadUrlRegex.SubexpNames() {
		switch subexpName {
		case "board":
			board = submatches[i]
			*pBoard = board
		case "no":
			no = submatches[i]
			*pNo = no
		}
	}
	return fmt.Sprintf("https://api.4chan.org/%s/res/%s.json", board, no)
}

func main() {
	var threadUrl string
	flag.StringVar(&threadUrl, "threadurl", "", "Url of the thread")
	flag.Parse()

	if threadUrl == "" {
		panic("-threadurl flag can't be empty")
	}

	if !threadUrlRegex.MatchString(threadUrl) {
		panic("Invalid thread URL")
	}

	var board, no string
	res, err := http.Get(jsonThreadUrl(threadUrl, &board, &no))
	if err != nil {
		panic(err)
	}

	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}

	var threadMap map[string][]map[string]interface{}
	err = json.Unmarshal(b, &threadMap)
	if err != nil {
		panic(err)
	}

	directory := fmt.Sprintf("./%s/No. %s", board, no)

	os.MkdirAll(directory, 0700)

	images := make([]Image, 0, 50)

	for _, postMap := range threadMap["posts"] {
		if _, ok := postMap["tim"]; ok {
			tim := int(postMap["tim"].(float64))
			ext := postMap["ext"].(string)

			image := Image{
				Name: fmt.Sprintf("%d%s", tim, ext),
				Url:  fmt.Sprintf("https://i.4cdn.org/%s/src/%d%s", board, tim, ext)}

			images = append(images, image)
		}
	}

	fmt.Println("Downloading", len(images), "images to", directory)

	finishedChan := make(chan string)
	finishedCount := 0

	for _, image := range images {
		go func(image Image) {
			if err := image.Download(directory); err != nil {
				finishedChan <- err.Error()
			} else {
				finishedChan <- image.Name + " has been downloaded"
			}
		}(image)
	}

	for message := range finishedChan {
		fmt.Println(message)
		finishedCount++

		if finishedCount == len(images) {
			close(finishedChan)
		}
	}

	fmt.Println("Done")
}