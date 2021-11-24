package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/allenai/autocut"
	"github.com/google/go-github/v39/github"
	"golang.org/x/oauth2"
)

func processArgs() (string, string, string, string, time.Duration, []string) {
	var owner string
	var repo string
	var title string
	var details string
	var durStr string
	var labelsStr string

	flag.StringVar(&owner, "owner", "", "Owner of the repo")
	flag.StringVar(&repo, "repo", "", "Name of the repo")
	flag.StringVar(&title, "title", "", "Title of the issue")
	flag.StringVar(&details, "details", "", "Details of the event")
	flag.StringVar(&durStr, "dur", "", "Duration threshold")
	flag.StringVar(&labelsStr, "labels", "", "Custom labels, separated by commas")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Cuts a new GitHub issue automatically, or updates an existing one.\n\n")
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	if owner == "" {
		fmt.Println("Error: Need an issue repo owner")
		fmt.Println()
		flag.Usage()
		os.Exit(1)
	}

	if repo == "" {
		fmt.Println("Error: Need an issue repo name")
		fmt.Println()
		flag.Usage()
		os.Exit(1)
	}

	if title == "" {
		fmt.Println("Error: Need an issue title")
		fmt.Println()
		flag.Usage()
		os.Exit(1)
	}

	if details == "" {
		fmt.Println("Error: Need an issue title")
		fmt.Println()
		flag.Usage()
		os.Exit(1)
	}

	if durStr == "" {
		fmt.Println("Error: Need a duration threshold")
		fmt.Println()
		flag.Usage()
		os.Exit(1)
	}

	var customLabels []string
	if labelsStr != "" {
		customLabels = strings.Split(labelsStr, ",")
	}

	dur, err := time.ParseDuration(durStr)
	if err != nil {
		fmt.Printf("Error: Duration invalid: %s\n", err.Error())
		fmt.Println()
		flag.Usage()
		os.Exit(1)
	}

	return owner, repo, title, details, dur, customLabels
}

func main() {
	log.SetFlags(0)

	owner, repo, title, details, dur, customLabels := processArgs()

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		fmt.Println("Need GitHub token in env var GITHUB_TOKEN")
		os.Exit(1)
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	ghClient := github.NewClient(tc)

	ac := &autocut.Autocut{
		Client:       ghClient,
		Owner:        owner,
		Repo:         repo,
		AgeThreshold: dur,
	}

	action, err := ac.Cut(ctx, title, details, customLabels)
	if err != nil {
		panic(err)
	}

	fmt.Println(action)
}
