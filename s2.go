package main

import (
	"bufio"
	"os"
	"sync"
	"strings"
	"regexp"
	"fmt"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	ch := make(chan string)
	wg := new(sync.WaitGroup)
	go Parser(ch,wg)
	i := 1
	for scanner.Scan() {
		if !Analyser(scanner.Text(),ch,wg) {
			fmt.Printf("Syntaxfel på rad %d\n", i)
			return
		}
		i++
	}
	wg.Wait()
}

func Analyser(input string, tokens chan<- string, wg *sync.WaitGroup) bool {
	words := strings.ToLower(input)
	current := " "
	regex,_ := regexp.Compile(`^(` +
		`(\s*(forw|back|left|right|down|up|color|rep|\.|"))|` +
		`(\s+(\d+|\#[a-z\d]{6})[\s\.])` +
	")$")
	comex,_ := regexp.Compile(`^\s*\%$`)
	for _,r := range words {
		current += (string(r))
		if regex.MatchString(current) {
			trimmed := strings.TrimSpace(current)
			wg.Add(1)
			if string(trimmed[len(trimmed)-1]) == "." {
				if trimmed != "." {
					wg.Add(1)
					tokens <- strings.TrimSuffix(trimmed,".")
				}
				tokens <- "."
			} else {
				tokens <- trimmed
			}
			current = ""
		} else if comex.MatchString(current) {
				return true
		}
	}
	if 	strings.TrimSpace(current) != "" {
		return false
	}
	return true
}

func Parser(tokens <-chan string, wg *sync.WaitGroup) {
	for token := range tokens {
		//TODO
		fmt.Println(token)
		wg.Done()
	}
}

/*
func Executor() { //TODO, kanal eller färdigt träd.
	//TODO
}
*/
