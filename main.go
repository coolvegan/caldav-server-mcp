package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "mcp":
			runMCP()
			return
		case "mcp-sse":
			runMCPSSE()
			return
		}
	}

	store, err := NewStore("./calendar")
	if err != nil {
		log.Fatalf("Store anlegen fehlgeschlagen: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && r.Method == "OPTIONS" {
			w.Header().Set("DAV", "1, 2, calendar-access")
			w.Header().Set("Allow", "OPTIONS, PROPFIND")
			return
		}
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/calendar/", 301)
			return
		}
		http.NotFound(w, r)
	})
	mux.Handle("/calendar/", caldavHandler(store))
	mux.HandleFunc("/.well-known/caldav", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("/calendar/"))
	})

	user := os.Getenv("CALDAV_USER")
	pass := os.Getenv("CALDAV_PASS")

	port := os.Getenv("CALDAV_PORT")
	if port == "" {
		port = "8080"
	}
	log.Println("Server startet auf :" + port)
	log.Fatal(http.ListenAndServe(":"+port, basicAuth(user, pass)(mux)))
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func caldavHandler(store *Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSuffix(r.URL.Path, "/")
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		log.Printf("[%s] %s\nBody: %s", r.Method, r.URL.Path, string(body))

		switch r.Method {
		case "OPTIONS":
			w.Header().Set("DAV", "1, 2, calendar-access")
			w.Header().Set("Allow", "OPTIONS, GET, PUT, DELETE, PROPFIND, REPORT")
			w.WriteHeader(200)

		case "PROPFIND":
			depth := r.Header.Get("Depth")
			if depth == "" {
				depth = "infinity"
			}

			w.Header().Set("Content-Type", "application/xml; charset=utf-8")
			w.WriteHeader(207)

			fmt.Fprint(w, xml.Header)
			fmt.Fprint(w, `<d:multistatus xmlns:d="DAV:" xmlns:cal="urn:ietf:params:xml:ns:caldav">`)

			// Calendar collection itself
			if path == "/calendar" {
				fmt.Fprint(w, `<d:response>`)
				fmt.Fprint(w, `<d:href>/calendar/</d:href>`)
				fmt.Fprint(w, `<d:propstat>`)
				fmt.Fprint(w, `<d:prop>`)
				fmt.Fprint(w, `<d:resourcetype><d:collection/><cal:calendar/></d:resourcetype>`)
				fmt.Fprint(w, `<d:displayname>Kalender</d:displayname>`)
				fmt.Fprint(w, `<d:current-user-principal><d:href>/</d:href></d:current-user-principal>`)
				fmt.Fprint(w, `<cal:calendar-home-set><d:href>/calendar/</d:href></cal:calendar-home-set>`)
				fmt.Fprint(w, `<cal:supported-calendar-component-set><cal:comp name="VEVENT"/></cal:supported-calendar-component-set>`)
				fmt.Fprint(w, `</d:prop>`)
				fmt.Fprint(w, `<d:status>HTTP/1.1 200 OK</d:status>`)
				fmt.Fprint(w, `</d:propstat>`)
				fmt.Fprint(w, `</d:response>`)
			}

			// Events (only for Depth:1 or infinity)
			if depth == "1" || depth == "infinity" {
				events, err := store.ListEvents()
				if err == nil {
					for _, e := range events {
						fmt.Fprint(w, `<d:response>`)
						fmt.Fprintf(w, `<d:href>/calendar/%s%s</d:href>`, e.UID, FILEEXT)
						fmt.Fprint(w, `<d:propstat>`)
						fmt.Fprint(w, `<d:prop>`)
						fmt.Fprintf(w, `<d:getcontenttype>text/calendar; charset=utf-8</d:getcontenttype>`)
						fmt.Fprintf(w, `<d:getetag>"%s"</d:getetag>`, e.UID)
						fmt.Fprint(w, `<d:resourcetype/>`)
						fmt.Fprint(w, `</d:prop>`)
						fmt.Fprint(w, `<d:status>HTTP/1.1 200 OK</d:status>`)
						fmt.Fprint(w, `</d:propstat>`)
						fmt.Fprint(w, `</d:response>`)
					}
				}
			}
			fmt.Fprint(w, `</d:multistatus>`)

		case "GET":
			name := strings.TrimPrefix(r.URL.Path, "/calendar/")
			if name == "" {
				w.Header().Set("Content-Type", "text/html")
				w.Write([]byte("<h1>CalDAV Server</h1>"))
				return
			}
			if !strings.HasSuffix(name, FILEEXT) {
				http.Error(w, "Not Found", 404)
				return
			}
			raw, err := os.ReadFile(filepath.Join(store.dir, name))
			if err != nil {
				http.Error(w, "Not Found", 404)
				return
			}
			w.Header().Set("Content-Type", "text/calendar")
			w.Write(raw)

		case "DELETE":
			name := strings.TrimPrefix(r.URL.Path, "/calendar/")
			if name == "" || !strings.HasSuffix(name, FILEEXT) {
				http.Error(w, "Not Found", 404)
				return
			}
			uid := strings.TrimSuffix(name, FILEEXT)
			if err := store.DeleteEvent(uid); err != nil {
				http.Error(w, "Not Found", 404)
				return
			}
			w.WriteHeader(204)

		case "PUT":
			e, err := parseEvent(string(body))
			if err != nil {
				http.Error(w, "Bad Request: "+err.Error(), 400)
				return
			}
			if err := store.SaveRaw(e.UID, body); err != nil {
				http.Error(w, "Internal Server Error", 500)
				return
			}
			w.WriteHeader(201)

		case "REPORT":
			bodyStr := string(body)

			// calendar-multiget: Thunderbird fragt Events per href
			if strings.Contains(bodyStr, "calendar-multiget") {
				type multiget struct {
					Hrefs []string `xml:"href"`
				}
				type multigetRoot struct {
					Hrefs []string `xml:"href"`
				}
				var mg multigetRoot
				if err := xml.Unmarshal(body, &mg); err != nil {
					http.Error(w, "Bad Request", 400)
					return
				}

				w.Header().Set("Content-Type", "application/xml; charset=utf-8")
				w.WriteHeader(207)
				fmt.Fprint(w, xml.Header)
				fmt.Fprint(w, `<d:multistatus xmlns:d="DAV:" xmlns:cal="urn:ietf:params:xml:ns:caldav">`)
				for _, href := range mg.Hrefs {
					name := strings.TrimPrefix(href, "/calendar/")
					raw, err := os.ReadFile(filepath.Join(store.dir, name))
					if err != nil {
						fmt.Fprintf(w, `<d:response><d:href>%s</d:href><d:status>HTTP/1.1 404 Not Found</d:status></d:response>`, href)
						continue
					}
					uid := strings.TrimSuffix(name, FILEEXT)
					fmt.Fprintf(w, `<d:response>`)
					fmt.Fprintf(w, `<d:href>%s</d:href>`, href)
					fmt.Fprint(w, `<d:propstat><d:prop>`)
					fmt.Fprintf(w, `<d:getetag>"%s"</d:getetag>`, uid)
					fmt.Fprintf(w, `<cal:calendar-data>%s</cal:calendar-data>`, xmlEscape(string(raw)))
					fmt.Fprint(w, `</d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat>`)
					fmt.Fprint(w, `</d:response>`)
				}
				fmt.Fprint(w, `</d:multistatus>`)
				return
			}

			var q struct {
				SyncToken struct {
					Token string `xml:",chardata"`
				} `xml:"sync-token"`
				Filter struct {
					CompFilter struct {
						CompFilter struct {
							TimeRange struct {
								Start string `xml:"start,attr"`
								End   string `xml:"end,attr"`
							} `xml:"time-range"`
						} `xml:"comp-filter"`
					} `xml:"comp-filter"`
				} `xml:"filter"`
			}
			if err := xml.Unmarshal(body, &q); err != nil {
				http.Error(w, "Bad Request", 400)
				return
			}

			if tokenStr := strings.TrimSpace(q.SyncToken.Token); tokenStr != "" {
				since, err := strconv.ParseInt(tokenStr, 10, 64)
				if err != nil {
					http.Error(w, "Bad Request", 400)
					return
				}
				events, newToken, err := store.SyncEvents(since)
				if err != nil {
					http.Error(w, "Internal Server Error", 500)
					return
				}
				w.Header().Set("Content-Type", "application/xml; charset=utf-8")
				w.WriteHeader(207)
				fmt.Fprint(w, xml.Header)
				fmt.Fprint(w, `<d:multistatus xmlns:d="DAV:" xmlns:cal="urn:ietf:params:xml:ns:caldav">`)
				for _, e := range events {
					fmt.Fprint(w, `<d:response>`)
					fmt.Fprintf(w, `<d:href>/calendar/%s%s</d:href>`, e.UID, FILEEXT)
					fmt.Fprint(w, `<d:propstat><d:prop>`)
					fmt.Fprintf(w, `<d:getetag>"%s"</d:getetag>`, e.UID)
					fmt.Fprint(w, `</d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat>`)
					fmt.Fprint(w, `</d:response>`)
				}
				fmt.Fprintf(w, `<d:sync-token>%d</d:sync-token>`, newToken)
				fmt.Fprint(w, `</d:multistatus>`)
				return
			}

			start := q.Filter.CompFilter.CompFilter.TimeRange.Start
			end := q.Filter.CompFilter.CompFilter.TimeRange.End
			if start == "" {
				http.Error(w, "Bad Request", 400)
				return
			}

			events, err := store.ListEvents()
			if err != nil {
				http.Error(w, "Internal Server Error", 500)
				return
			}
			w.Header().Set("Content-Type", "application/xml; charset=utf-8")
			w.WriteHeader(207)
			fmt.Fprint(w, xml.Header)
			fmt.Fprint(w, `<d:multistatus xmlns:d="DAV:" xmlns:cal="urn:ietf:params:xml:ns:caldav">`)
			for _, e := range events {
				if e.DTEND >= start && (end == "" || e.DTSTART <= end) {
					fmt.Fprint(w, `<d:response>`)
					fmt.Fprintf(w, `<d:href>/calendar/%s%s</d:href>`, e.UID, FILEEXT)
					fmt.Fprint(w, `<d:propstat><d:prop>`)
					fmt.Fprintf(w, `<d:getetag>"%s"</d:getetag>`, e.UID)
					fmt.Fprint(w, `</d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat>`)
					fmt.Fprint(w, `</d:response>`)
				}
			}
			fmt.Fprint(w, `</d:multistatus>`)

		default:
			http.Error(w, "Method Not Allowed", 405)
		}
	})
}
