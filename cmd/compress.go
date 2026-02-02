package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/tinytui/tinytui/internal/history"
	"github.com/tinytui/tinytui/internal/pipeline"
	"github.com/tinytui/tinytui/internal/scanner"
)

var (
	stdinFlag     bool
	outputDirFlag string
	suffixFlag    string
)

var compressCmd = &cobra.Command{
	Use:   "compress [paths...]",
	Short: "Compress images via CLI",
	Run: func(cmd *cobra.Command, args []string) {
		// Check API Key
		if !cfg.IsConfigured() {
			fmt.Println("Error: API Key not configured. Run 'tinytui config set-key <KEY>' first.")
			os.Exit(1)
		}

		paths := args
		if stdinFlag {
			// Read from stdin
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line != "" {
					paths = append(paths, line)
				}
			}
		}

		if len(paths) == 0 {
			cmd.Help()
			return
		}

		// Scan
		scanRes, err := scanner.Scan(paths, true) // recurse by default for CLI? Prompt doesn't specify default recursion for CLI, but for UI it says "Options: [x] recursive". Let's assume true or add flag.
		// "B) Paste Path / Glob ... ./imgs/*.png"
		if err != nil {
			fmt.Printf("Scan error: %v\n", err)
			os.Exit(1)
		}
		
		if len(scanRes.Errors) > 0 {
			for _, e := range scanRes.Errors {
				fmt.Printf("Warning: %v\n", e)
			}
		}
		
		if len(scanRes.Images) == 0 {
			fmt.Println("No images found.")
			return
		}

		// Override config if flags set
		if outputDirFlag != "" {
			cfg.OutputMode = "directory"
			cfg.OutputDir = outputDirFlag
		}
		if suffixFlag != "" {
			cfg.Suffix = suffixFlag
		} else {
			// If not set via flag, keep config default
		}

		// Setup Pipeline
		p := pipeline.New(cfg, cfg.APIKey)
		p.Configure(2) // Default concurr
		p.Start()
		defer p.Stop()

		// Add files
		p.AddFiles(scanRes.Images)

		// Setup History Manager
		hMgr, _ := history.New() // Ignore error, best effort logging

		// Monitor Progress
		// Table output: | Status | File | Before | After | Saved % |
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "Status\tFile\tBefore\tAfter\tSaved %\tError")
		
		totalBefore := int64(0)
		totalAfter := int64(0)
		totalSaved := int64(0)
		processedCount := 0
		errorCount := 0

		// We need to track how many jobs completed to exit
		// Pipeline doesn't auto-close Updates when empty.
		// We know how many we added.
		target := len(scanRes.Images)
		done := 0

		for job := range p.Updates() {
			if job.Status == pipeline.StatusDone || job.Status == pipeline.StatusFailed {
				done++
				
				statusParams := job.Status
				errStr := ""
				if job.Error != nil {
					errStr = job.Error.Error()
					errorCount++
				} else {
					processedCount++
					totalBefore += job.OriginalSize
					totalAfter += int64(job.CompressedSize) // int64? Fixed job struct type mismatch in mind? Job has int64.
					totalSaved += job.SavedBytes
					
					// Log to history
					if hMgr != nil {
						hMgr.Add(&history.Record{
							Timestamp:    time.Now(),
							File:         job.FilePath,
							BeforeSize:   job.OriginalSize,
							AfterSize:    job.CompressedSize,
							SavedBytes:   job.SavedBytes,
							SavedPercent: job.SavedPercent,
							Status:       "success",
						})
					}
				}
				
				// Print Row
				// In CLI mode with many files, we probably want line-by-line output?
				// "Display table" -> usually implies buffered or updated. But for "script friendly", line by line is better or final table.
				// "For each file: Read ... Calculate ... Display table". implies streaming table rows.
				// "Final summary panel" at end.
				
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%.1f%%\t%s\n",
					string(statusParams),
					shortPath(job.FilePath),
					formatBytes(job.OriginalSize),
					formatBytes(job.CompressedSize),
					job.SavedPercent,
					errStr,
				)
				w.Flush()

				if done >= target {
					break // All processed
				}
			}
		}

		// Final Summary
		fmt.Println("--------------------------------------------------")
		fmt.Printf("Compression complete ✔\n")
		fmt.Printf("Files processed : %d\n", processedCount)
		fmt.Printf("Total before    : %s\n", formatBytes(totalBefore))
		fmt.Printf("Total after     : %s\n", formatBytes(totalAfter))
		fmt.Printf("Total saved     : %s (%.0f%%)\n", formatBytes(totalSaved), float64(totalSaved)/float64(totalBefore)*100)
		fmt.Printf("Errors          : %d\n", errorCount)
	},
}

func init() {
	rootCmd.AddCommand(compressCmd)
	compressCmd.Flags().BoolVar(&stdinFlag, "stdin", false, "Read paths from stdin")
	compressCmd.Flags().StringVar(&outputDirFlag, "output-dir", "", "Output directory")
	compressCmd.Flags().StringVar(&suffixFlag, "suffix", "", "Filename suffix")
}

func shortPath(p string) string {
	if len(p) > 30 {
		return "..." + p[len(p)-27:]
	}
	return p
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMgtpe"[exp])
}
