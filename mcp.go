package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func runMCP() {
	store, err := NewStore(os.Getenv("CALDAV_DIR"))
	if err != nil {
		log.Fatalf("Store: %v", err)
	}

	s := createMCPServer(store)

	log.Println("MCP-Server gestartet (STDIO)")
	if err := server.ServeStdio(s); err != nil {
		log.Fatal(err)
	}
}

func runMCPSSE() {
	store, err := NewStore(os.Getenv("CALDAV_DIR"))
	if err != nil {
		log.Fatalf("Store: %v", err)
	}

	s := createMCPServer(store)

	sseServer := server.NewSSEServer(s,
		server.WithSSEContextFunc(func(ctx context.Context, r *http.Request) context.Context {
			return ctx
		}),
	)

	port := "8081"
	if p := os.Getenv("CALDAV_MCP_PORT"); p != "" {
		port = p
	}
	addr := "127.0.0.1:" + port
	log.Println("MCP-Server gestartet (SSE) auf", addr)
	log.Fatal(sseServer.Start(addr))
}

func createMCPServer(store *Store) *server.MCPServer {
	s := server.NewMCPServer(
		"caldav",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	listTool := mcp.NewTool("list_events",
		mcp.WithDescription("Listet alle Kalender-Termine"),
	)
	s.AddTool(listTool, func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		events, err := store.ListEvents()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if len(events) == 0 {
			return mcp.NewToolResultText("Keine Termine."), nil
		}
		var sb strings.Builder
		for _, e := range events {
			sb.WriteString(fmt.Sprintf("%s: %s (%s - %s)", e.UID, e.SUMMARY, e.DTSTART, e.DTEND))
			if e.LOCATION != "" {
				sb.WriteString(fmt.Sprintf(" @ %s", e.LOCATION))
			}
			if e.DESCRIPTION != "" {
				sb.WriteString(fmt.Sprintf("\n  %s", e.DESCRIPTION))
			}
			sb.WriteString("\n")
		}
		return mcp.NewToolResultText(sb.String()), nil
	})

	createTool := mcp.NewTool("create_event",
		mcp.WithDescription("Erstellt einen neuen Kalender-Termin"),
		mcp.WithString("summary", mcp.Required(), mcp.Description("Titel des Termins")),
		mcp.WithString("dtstart", mcp.Required(), mcp.Description("Startzeit (YYYYMMDDTHHMMSSZ)")),
		mcp.WithString("dtend", mcp.Required(), mcp.Description("Endzeit (YYYYMMDDTHHMMSSZ)")),
		mcp.WithString("location", mcp.Description("Ort des Termins (optional)")),
		mcp.WithString("description", mcp.Description("Beschreibung des Termins (optional)")),
	)
	s.AddTool(createTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.Params.Arguments.(map[string]interface{})
		e := Event{
			UID:     uuid.New().String(),
			SUMMARY: args["summary"].(string),
			DTSTART: args["dtstart"].(string),
			DTEND:  args["dtend"].(string),
		}
		if loc, ok := args["location"].(string); ok {
			e.LOCATION = loc
		}
		if desc, ok := args["description"].(string); ok {
			e.DESCRIPTION = desc
		}
		if desc, ok := args["description"].(string); ok {
			e.DESCRIPTION = desc
		}
		data := fmt.Sprintf("BEGIN:VCALENDAR\nBEGIN:VEVENT\nUID:%s\nDTSTART:%s\nDTEND:%s\nSUMMARY:%s\nLOCATION:%s\nDESCRIPTION:%s\nEND:VEVENT\nEND:VCALENDAR\n",
			e.UID, e.DTSTART, e.DTEND, e.SUMMARY, e.LOCATION, e.DESCRIPTION)
		if err := store.SaveRaw(e.UID, []byte(data)); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Termin erstellt: %s", e.UID)), nil
	})

	deleteTool := mcp.NewTool("delete_event",
		mcp.WithDescription("Loescht einen Termin"),
		mcp.WithString("uid", mcp.Required(), mcp.Description("UID des Termins")),
	)
	s.AddTool(deleteTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		uid := strings.TrimSpace(req.Params.Arguments.(map[string]interface{})["uid"].(string))
		if err := os.Remove(filepath.Join(store.dir, uid+FILEEXT)); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText("Termin geloescht."), nil
	})

	return s
}
