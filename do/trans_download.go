package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	apptranslatoServer  = "https://www.apptranslator.org"
	translationsTxtPath = filepath.Join("translations", "translations.txt")
)

func getTransSecret() string {
	v := os.Getenv("TRANS_UPLOAD_SECRET")
	panicIf(v == "", "must set TRANS_UPLOAD_SECRET env variable")
	return v
}

func printSusTranslations(d []byte) {
	a := strings.Split(string(d), "\n")
	currString := ""
	isSus := func(s string) bool {
		/*
			if strings.Contains(s, `\n\n`) {
				return true
			}
		*/
		if strings.HasPrefix(s, `\n`) || strings.HasSuffix(s, `\n`) {
			return true
		}
		if strings.HasPrefix(s, `\r`) || strings.HasSuffix(s, `\r`) {
			return true
		}
		return false
	}

	for _, s := range a {
		if strings.HasPrefix(s, ":") {
			currString = s[1:]
			continue
		}
		if isSus(s) {
			fmt.Printf("Suspicious translation:\n%s\n%s\n\n", currString, s)
		}
	}
}

func downloadTranslationsMust() []byte {
	timeStart := time.Now()
	defer func() {
		fmt.Printf("downloadTranslations() finished in %s\n", time.Since(timeStart))
	}()
	strs := extractStringsFromCFilesNoPaths()
	sort.Strings(strs)
	fmt.Printf("uploading %d strings for translation\n", len(strs))
	secret := getTransSecret()
	uri := apptranslatoServer + "/api/dltransfor?app=SumatraPDF&secret=" + secret
	s := strings.Join(strs, "\n")
	body := strings.NewReader(s)
	req, err := http.NewRequest(http.MethodPost, uri, body)
	must(err)
	client := http.DefaultClient
	rsp, err := client.Do(req)
	must(err)
	panicIf(rsp.StatusCode != http.StatusOK)
	d, err := io.ReadAll(rsp.Body)
	must(err)
	return d
}

/*
The file looks like:

AppTranslator: SumatraPDF
608ebc3039db395ff05d3d5d950afdd65a233c58
:&About
af:&Omtrent
am:&Ծրագրի մասին
*/
func splitIntoPerLangFiles(d []byte) {
	a := strings.Split(string(d), "\n")
	a = a[2:]
	perLang := make(map[string]map[string]string)
	allStrings := []string{}
	currString := "" // string we're currently processing

	addLangTrans := func(lang, trans string) {
		m := perLang[lang]
		if m == nil {
			m = make(map[string]string)
			perLang[lang] = m
		}
		m[currString] = trans
	}

	// build perLang maps
	for _, s := range a {
		if len(s) == 0 {
			// can happen at the end of the file
			continue
		}
		if strings.HasPrefix(s, ":") {
			currString = s[1:]
			allStrings = append(allStrings, currString)
			continue
		}
		parts := strings.SplitN(s, ":", 2)
		lang := parts[0]
		panicIf(len(lang) > 5)
		panicIf(len(parts) == 1, "parts: '%s'\n", parts)
		trans := parts[1]
		addLangTrans(lang, trans)
	}

	for lang, m := range perLang {
		a := []string{}
		sort.Slice(allStrings, func(i, j int) bool {
			s1 := allStrings[i]
			s2 := allStrings[j]
			s1IsTranslated := m[s1] != ""
			s2IsTranslated := m[s2] != ""
			if !s1IsTranslated && s2IsTranslated {
				return true
			}
			if s1IsTranslated && !s2IsTranslated {
				return false
			}
			return s1 < s2
		})
		for _, s := range allStrings {
			a = append(a, ":"+s)
			trans := m[s]
			panicIf(strings.Contains(trans, "\n"))
			if len(trans) == 0 {
				continue
			}
			a = append(a, trans)
		}
		// TODO: sort so that untranslated strings are at start
		s := strings.Join(a, "\n")
		path := filepath.Join("translations", lang+".txt")
		writeFileMust(path, []byte(s))
		logf(ctx(), "Wrote: '%s'\n", path)
	}
}

func downloadTranslations() bool {
	d := downloadTranslationsMust()

	curr := readFileMust(translationsTxtPath)
	if bytes.Equal(d, curr) {
		fmt.Printf("Translations didn't change\n")
		//TODO: for now to force splitting into per-lang files
		// return false
	}

	writeFileMust(translationsTxtPath, d)
	// TODO: save ~400k in uncompressed binary by
	// saving as gzipped and embedding that in the exe
	//u.WriteFileGzipped(translationsTxtPath+".gz", d)
	splitIntoPerLangFiles(d)
	fmt.Printf("Wrote response of size %d to %s\n", len(d), translationsTxtPath)
	printSusTranslations(d)
	return false
}

// TODO:
// - generate translations/status.md file that shows how many
//   strings untranslated per language and links to their files
// - do this when updating from soource:
//	 - read current per-lang translations
//   - extract strings from source
//   - remove no longer needed
//   - add new ones
//   - re-save per-lang files
//   - save no longer needeed in obsolete.txt
