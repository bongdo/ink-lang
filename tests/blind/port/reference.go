package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"unicode"
)

type result struct {
	counts map[string]int
	err    error
}

func count(r io.Reader) (map[string]int, error) {
	counts := map[string]int{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	sc.Split(bufio.ScanWords)
	for sc.Scan() {
		w := strings.TrimFunc(sc.Text(), func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '\''
		})
		if len([]rune(w)) < 2 {
			continue
		}
		counts[strings.ToLower(w)]++
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return counts, nil
}

func main() {
	n := flag.Int("n", 10, "how many words to print")
	min := flag.Int("min", 1, "minimum count to print")
	flag.Parse()

	paths := flag.Args()
	if len(paths) == 0 {
		paths = []string{""}
	}

	results := make([]result, len(paths))
	var wg sync.WaitGroup
	for i, path := range paths {
		wg.Add(1)
		go func(i int, path string) {
			defer wg.Done()
			var r io.Reader = os.Stdin
			if path != "" {
				f, err := os.Open(path)
				if err != nil {
					results[i] = result{err: err}
					return
				}
				defer f.Close()
				r = f
			}
			counts, err := count(r)
			results[i] = result{counts: counts, err: err}
		}(i, path)
	}
	wg.Wait()

	total := map[string]int{}
	for _, res := range results {
		if res.err != nil {
			fmt.Fprintln(os.Stderr, "wordfreq:", res.err)
			os.Exit(1)
		}
		for w, c := range res.counts {
			total[w] += c
		}
	}

	type entry struct {
		word  string
		count int
	}
	entries := make([]entry, 0, len(total))
	for w, c := range total {
		if c >= *min {
			entries = append(entries, entry{w, c})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].word < entries[j].word
	})
	if *n < len(entries) {
		entries = entries[:*n]
	}
	for _, e := range entries {
		fmt.Printf("%d\t%s\n", e.count, e.word)
	}
}
