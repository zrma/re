package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func main() {
	targetPath := ""
	flag.StringVar(&targetPath, "t", "", "target path")

	flag.Parse()

	if targetPath == "" {
		targetPath = "."
	}

	changeExtToLower(targetPath)

	movieExtList := map[string]bool{"avi": true, "mkv": true, "mp4": true}
	subtitleExtList := map[string]bool{"srt": true, "ass": true, "smi": true}

	episodeToMovieMap := map[string]string{}
	episodeToSubtitleMap := map[string][]string{}

	var movies, subtitle []string

	err := filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Dir(path) != targetPath {
			return nil
		}

		ext := filepath.Ext(path)
		ext = ext[1:]
		if movieExtList[ext] {
			movies = append(movies, filepath.Base(path))
		}
		if subtitleExtList[ext] {
			subtitle = append(subtitle, path)
		}
		return nil
	})
	if err != nil {
		log.Fatalln(err)
	}

	sort.Strings(movies)
	sort.Strings(subtitle)

	for _, movie := range movies {
		fileName := filepath.Base(movie)
		fileName = strings.TrimRight(fileName, filepath.Ext(fileName))
		episode := episodeFromMovie(fileName)
		if episode == "" {
			log.Println("[mov] episode is empty", movie)
			continue
		}
		//log.Printf("episode: %s, movie: %s", episode, movie)
		episodeToMovieMap[episode] = fileName
	}

	for _, sub := range subtitle {
		fileName := filepath.Base(sub)
		episode := episodeFromSubtitle(fileName)
		if episode == "" {
			log.Println("[sub] episode is empty", sub)
			continue
		}
		//log.Printf("episode: %s, sub: %s", episode, sub)
		episodeToSubtitleMap[episode] = append(episodeToSubtitleMap[episode], sub)
	}

	episodes := make([]string, 0, len(episodeToMovieMap))
	for episode := range episodeToMovieMap {
		episodes = append(episodes, episode)
	}
	sort.Strings(episodes)

	changeFiles(false, episodes, episodeToMovieMap, episodeToSubtitleMap)

	fmt.Println("Do you want to rename? (y/n)")

	var input string
	_, _ = fmt.Scanln(&input)
	if strings.ToLower(input) != "y" {
		fmt.Printf("Canceled")
		return
	}

	changeFiles(true, episodes, episodeToMovieMap, episodeToSubtitleMap)
	fmt.Println("Done!")
}

func changeFiles(rename bool, episodes []string, episodeToMovieMap map[string]string, episodeToSubtitleMap map[string][]string) {
	for _, episode := range episodes {
		movie := episodeToMovieMap[episode]
		subs := episodeToSubtitleMap[episode]

		for _, sub := range subs {
			ext := filepath.Ext(sub)

			dir := filepath.Dir(sub)
			newSubName := movie + ext
			newSubPath := filepath.Join(dir, newSubName)

			if rename {
				err := os.Rename(sub, newSubPath)
				if err != nil {
					log.Fatalln(err)
				}
			} else {
				fmt.Printf("%s -> %s\n", sub, newSubName)
			}
		}
	}
}

func episodeFromMovie(fileName string) string {
	regex := regexp.MustCompile(`- \d{2} (END |)\(`)
	episode := regex.FindString(fileName)
	episode = strings.TrimLeft(episode, "- ")
	episode = strings.TrimRight(episode, " (")
	episode = strings.TrimRight(episode, " END")
	episode = strings.Trim(episode, " ")
	return episode
}

func episodeFromSubtitle(name string) string {
	seasonTypeRegex := regexp.MustCompile(`S\d{2}E\d{2}`)
	seasonTypeName := seasonTypeRegex.FindString(name)
	if seasonTypeName != "" {
		seasonTypeName = seasonTypeName[4:]
		return seasonTypeName
	}

	movieLikedTypeRegex := regexp.MustCompile(`\d{2} (END |)\(`)
	movieLikedTypeName := movieLikedTypeRegex.FindString(name)
	if movieLikedTypeName != "" {
		movieLikedTypeName = strings.TrimRight(movieLikedTypeName, " (")
		movieLikedTypeName = strings.TrimRight(movieLikedTypeName, " END")
		movieLikedTypeName = strings.Trim(movieLikedTypeName, " ")
		return movieLikedTypeName
	}

	koreanTypeRegex := regexp.MustCompile(`\d{1,2}화`)
	koreanTypeName := koreanTypeRegex.FindString(name)
	if koreanTypeName != "" {
		koreanTypeName = strings.TrimRight(koreanTypeName, "화")
		koreanTypeName = strings.Trim(koreanTypeName, " ")

		if len(koreanTypeName) == 1 {
			koreanTypeName = "0" + koreanTypeName
		}
		return koreanTypeName
	}

	endWithTypeRegex := regexp.MustCompile(`\d{2}\.`)
	endWithTypeName := endWithTypeRegex.FindString(name)
	if endWithTypeName != "" {
		endWithTypeName = strings.TrimRight(endWithTypeName, ".")
		endWithTypeName = strings.Trim(endWithTypeName, " ")
		return endWithTypeName
	}
	return ""
}

func changeExtToLower(targetPath string) {
	movieExtList := map[string]bool{
		"avi": true, "mkv": true, "mp4": true,
		"AVI": true, "MKV": true, "MP4": true,
	}
	subtitleExtList := map[string]bool{
		"srt": true, "ass": true, "smi": true,
		"SRT": true, "ASS": true, "SMI": true,
	}
	err := filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Dir(path) != targetPath {
			return nil
		}

		ext := filepath.Ext(path)
		ext = ext[1:]
		if movieExtList[ext] {
			if ext != strings.ToLower(ext) {
				ext = strings.ToLower(ext)
				newPath := strings.TrimRight(path, filepath.Ext(path)) + "." + ext
				err := os.Rename(path, newPath)
				if err != nil {
					log.Fatalln(err)
				}
			}
		}
		if subtitleExtList[ext] {
			if ext != strings.ToLower(ext) {
				ext = strings.ToLower(ext)
				newPath := strings.TrimRight(path, filepath.Ext(path)) + "." + ext
				err := os.Rename(path, newPath)
				if err != nil {
					log.Fatalln(err)
				}
			}
		}
		return nil
	})
	if err != nil {
		log.Fatalln(err)
	}
}