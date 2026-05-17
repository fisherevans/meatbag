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
		Long: "Scans every active and archived list, collects referenced secret " +
			"and blob refs, then deletes anything in Keychain (service=meatbag) " +
			"or ~/.meatbag/blobs/ that isn't referenced. --dry-run reports without " +
			"deleting.",
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
			for _, l := range lists {
				collectRefs(l.Items, refSecrets, refBlobs)
			}

			// Reconcile Keychain.
			allKC, err := secrets.ListAll()
			if err != nil {
				return fmt.Errorf("keychain list: %w", err)
			}
			var orphanSecrets []secrets.Ref
			for _, r := range allKC {
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
				"orphan_blobs":   len(orphanBlobs),
				"dry_run":        dryRun,
			}
			text := fmt.Sprintf("%d orphan secrets, %d orphan blobs", len(orphanSecrets), len(orphanBlobs))
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
