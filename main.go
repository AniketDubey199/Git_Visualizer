package main

import "flag"

func main() {
	var folder string
	var email string

	flag.StringVar(&folder, "add", "", " add a new folder to scan for git repositories")
	flag.StringVar(&email, "email", "aniketdubey0124@gmail.com", "the email to scan")

	flag.Parse()

	if folder != "" {

	}

}
