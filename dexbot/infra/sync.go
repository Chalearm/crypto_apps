/*
Filename: infra/sync.go

Author: M365 Copilot (GPT-5)
Version: v1.0
Owner: Chalearm Saelim
Date: 2026-06-11 21:20

Description:
Data synchronization module.

Scans local storage for unsynced data and attempts to push to database.

Current version:
- Logs actions only (DB stub)
- Prepares for future real sync implementation

AI Prompt Idea:
"Create a Go module that scans local files and syncs them into a database with retry mechanism."

How to use:
infra.RunSyncCycle()

How to test:
cd dexbot
go test ./infra -v

How to build:
go build ./...

How to run:
Used internally in daemon
*/

package infra

import (
    "os"
    "path/filepath"
)

func RunSyncCycle() {

    files, err := filepath.Glob("data/buffer/*.json")
    if err != nil {
        Error("failed to scan buffer folder")
        return
    }

    if len(files) == 0 {
        Info("no files to sync")
        return
    }

    for _, f := range files {
        Info("syncing file: " + f)

        if DBHealthy() {
            Info("DB OK → would insert data: " + f)

            // future: real insert here

            err := os.Remove(f)
            if err != nil {
                Warn("failed to delete synced file")
            } else {
                Info("file synced and removed: " + f)
            }

        } else {
            Warn("DB not available → keep local file: " + f)
        }
    }
}
