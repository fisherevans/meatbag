package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/fisherevans/meatbag/internal/blobs"
	"github.com/fisherevans/meatbag/internal/secrets"
	"github.com/fisherevans/meatbag/internal/store"
)

func newGCCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "gc",
		Short: "Reconcile Keychain entries and blob files against current lists",
		Long: "Scans every active and archived list in the current $MEATBAG_HOME, " +
			"collects referenced secret and blob refs, then deletes orphaned " +
			"Keychain entries (service=meatbag) whose list-id belongs to a list " +
			"in this home, plus orphaned files in $MEATBAG_HOME/blobs/. Keychain " +
			"entries belonging to lists in other homes are reported as 'alien' " +
			"and left alone. --dry-run reports without deleting.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			lists, err := s.ListAll("all")
			if err != nil {
				return err
			}
			refSecrets := map[string]bool{}
			refBlobs := map[string]bool{}
			homeListIDs := map[string]bool{}
			for _, l := range lists {
				homeListIDs[l.ID] = true
				collectRefs(l.Items, refSecrets, refBlobs)
			}

			// Reconcile Keychain. Scope to entries whose list-id belongs to a
			// list in the current $MEATBAG_HOME. Entries from other homes
			// (e.g. another working dir or a real-user home when running tests
			// in an isolated tmp dir) are treated as "alien" and left alone.
			allKC, err := secrets.ListAll()
			if err != nil {
				return fmt.Errorf("keychain list: %w", err)
			}
			var orphanSecrets []secrets.Ref
			var alienSecrets int
			for _, r := range allKC {
				if !homeListIDs[r.ListID] {
					alienSecrets++
					continue
				}
				if !refSecrets[r.Account()] {
					orphanSecrets = append(orphanSecrets, r)
				}
			}

			// Reconcile blobs.
			bs, _ := blobs.New(s.BlobsDir())
			allBlobs, _ := bs.ListAll()
			var orphanBlobs []string
			for _, sha := range allBlobs {
				if !refBlobs[sha] {
					orphanBlobs = append(orphanBlobs, sha)
				}
			}

			if !dryRun {
				for _, r := range orphanSecrets {
					_ = secrets.Delete(r)
				}
				for _, sha := range orphanBlobs {
					_ = bs.Delete(sha)
				}
			}

			out := map[string]any{
				"orphan_secrets": len(orphanSecrets),
				"alien_secrets":  alienSecrets,
				"orphan_blobs":   len(orphanBlobs),
				"dry_run":        dryRun,
			}
			text := fmt.Sprintf("%d orphan secrets", len(orphanSecrets))
			if alienSecrets > 0 {
				text += fmt.Sprintf(", %d alien (left alone)", alienSecrets)
			}
			text += fmt.Sprintf(", %d orphan blobs", len(orphanBlobs))
			if dryRun {
				text += " (dry-run, nothing deleted)"
			} else {
				text += " (deleted)"
			}
			return emit(out, text)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report without deleting")
	return cmd
}

func collectRefs(items []*store.Item, refSecrets, refBlobs map[string]bool) {
	for _, it := range items {
		for _, v := range it.InputValues {
			if v.SecretRef != "" {
				if r, err := secrets.ParseRef(v.SecretRef); err == nil {
					refSecrets[r.Account()] = true
				}
			}
			if v.BlobRef != "" {
				if sha, err := blobs.ParseRef(v.BlobRef); err == nil {
					refBlobs[sha] = true
				}
			}
		}
		collectRefs(it.Children, refSecrets, refBlobs)
	}
}
