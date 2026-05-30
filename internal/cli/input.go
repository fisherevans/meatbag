package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/fisherevans/meatbag/internal/blobs"
	"github.com/fisherevans/meatbag/internal/secrets"
	"github.com/fisherevans/meatbag/internal/store"
	"github.com/fisherevans/meatbag/internal/tree"
)

func newInputCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "input",
		Short: "Get/set structured input values on items",
	}
	cmd.AddCommand(inputGetCmd(), inputSetCmd(), inputClearCmd())
	return cmd
}

// findInputSchema returns the schema for the named field, or nil if not declared.
func findInputSchema(it *store.Item, field string) *store.Input {
	for i := range it.Inputs {
		if it.Inputs[i].Name == field {
			return &it.Inputs[i]
		}
	}
	return nil
}

func inputGetCmd() *cobra.Command {
	var reveal bool
	cmd := &cobra.Command{
		Use:   "get <list> <item> <field>",
		Short: "Read an input value. Secrets are redacted unless --reveal.",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			path, _, err := s.FindPath(args[0])
			if err != nil {
				return fail("get", err)
			}
			l, err := s.LoadList(path)
			if err != nil {
				return err
			}
			it, ok := tree.Resolve(l, args[1])
			if !ok {
				return fmt.Errorf("item not found: %s", args[1])
			}
			v, ok := it.InputValues[args[2]]
			if !ok || !v.HasValue {
				if gFlags.JSON {
					return emitJSON(map[string]any{"field": args[2], "has_value": false})
				}
				fmt.Printf("%s: (unset)\n", args[2])
				return nil
			}
			schema := findInputSchema(it, args[2])
			out := map[string]any{
				"field":     args[2],
				"has_value": true,
			}
			if schema != nil {
				out["type"] = schema.Type
			}
			if v.SecretRef != "" {
				out["secret_ref"] = v.SecretRef
				if reveal {
					ref, err := secrets.ParseRef(v.SecretRef)
					if err != nil {
						return err
					}
					val, err := secrets.Get(ref)
					if err != nil {
						if errors.Is(err, secrets.ErrNotFound) {
							// The YAML says has_value=true but the Keychain entry is
							// gone. Emit a machine-readable signal rather than a bare
							// stderr error so piped jq doesn't break.
							out["has_value"] = false
							out["keychain_missing"] = true
							if gFlags.JSON {
								return emitJSON(out)
							}
							fmt.Fprintf(os.Stderr, "warning: keychain entry missing for %s - secret must be re-entered\n", args[2])
							return nil
						}
						return fmt.Errorf("read secret: %w", err)
					}
					out["value"] = val
				}
			}
			if v.BlobRef != "" {
				out["blob_ref"] = v.BlobRef
				out["filename"] = v.Filename
				out["size"] = v.Size
				bs, _ := blobs.New(s.BlobsDir())
				if reveal && bs != nil {
					sha, _ := blobs.ParseRef(v.BlobRef)
					out["path"] = bs.Path(sha)
				}
			}
			if v.Value != nil {
				out["value"] = v.Value
			}
			if gFlags.JSON {
				return emitJSON(out)
			}
			// Text output
			switch {
			case v.SecretRef != "":
				if reveal {
					fmt.Println(out["value"])
				} else {
					fmt.Println("(secret; pass --reveal to print)")
				}
			case v.BlobRef != "":
				fmt.Printf("file: %s (%d bytes)\n", v.Filename, v.Size)
				if reveal {
					fmt.Printf("path: %s\n", out["path"])
				}
			default:
				fmt.Println(out["value"])
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&reveal, "reveal", false, "include secret/file contents in output")
	return cmd
}

func inputSetCmd() *cobra.Command {
	var (
		valueStr  string
		filePath  string
		fromStdin bool
	)
	cmd := &cobra.Command{
		Use:   "set <list> <item> <field>",
		Short: "Write an input value",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			modes := 0
			if cmd.Flags().Changed("value") {
				modes++
			}
			if filePath != "" {
				modes++
			}
			if fromStdin {
				modes++
			}
			if modes != 1 {
				return fmt.Errorf("specify exactly one of --value, --file, --stdin")
			}
			s, err := openStore()
			if err != nil {
				return err
			}
			path, _, err := s.FindPath(args[0])
			if err != nil {
				return fail("set", err)
			}
			l, err := s.LoadList(path)
			if err != nil {
				return err
			}
			it, ok := tree.Resolve(l, args[1])
			if !ok {
				return fmt.Errorf("item not found: %s", args[1])
			}
			schema := findInputSchema(it, args[2])
			if schema == nil {
				return fmt.Errorf("item has no input field %q", args[2])
			}

			bs, err := blobs.New(s.BlobsDir())
			if err != nil {
				return err
			}

			var newVal store.InputValue
			switch schema.Type {
			case "password":
				val, err := readScalar(valueStr, fromStdin, "")
				if err != nil {
					return err
				}
				ref, err := secrets.Set(l.ID, it.ID, schema.Name, val)
				if err != nil {
					return fmt.Errorf("keychain: %w", err)
				}
				newVal = store.InputValue{SecretRef: ref, HasValue: true}
			case "file":
				if filePath == "" && !fromStdin {
					return fmt.Errorf("--file or --stdin required for file inputs")
				}
				var r io.Reader
				var filename string
				if fromStdin {
					r = os.Stdin
					filename = ""
				} else {
					f, err := os.Open(filePath)
					if err != nil {
						return err
					}
					defer f.Close()
					r = f
					filename = filepath.Base(filePath)
				}
				sha, size, err := bs.Write(r)
				if err != nil {
					return err
				}
				newVal = store.InputValue{
					BlobRef:  blobs.BuildRef(sha),
					Filename: filename,
					Size:     size,
					HasValue: true,
				}
			case "number":
				val, err := readScalar(valueStr, fromStdin, "")
				if err != nil {
					return err
				}
				n, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
				if err != nil {
					return fmt.Errorf("number parse: %w", err)
				}
				newVal = store.InputValue{Value: n, HasValue: true}
			case "checkbox":
				val, err := readScalar(valueStr, fromStdin, "")
				if err != nil {
					return err
				}
				b, err := strconv.ParseBool(strings.TrimSpace(val))
				if err != nil {
					return fmt.Errorf("checkbox parse: %w", err)
				}
				newVal = store.InputValue{Value: b, HasValue: true}
			case "multiselect":
				val, err := readScalar(valueStr, fromStdin, "")
				if err != nil {
					return err
				}
				parts := strings.Split(val, ",")
				for i := range parts {
					parts[i] = strings.TrimSpace(parts[i])
				}
				newVal = store.InputValue{Value: parts, HasValue: true}
			default: // text, textarea, url, select, radio, markdown
				val, err := readScalar(valueStr, fromStdin, filePath)
				if err != nil {
					return err
				}
				newVal = store.InputValue{Value: val, HasValue: true}
			}

			err = s.Update(args[0], func(l *store.List) error {
				it, ok := tree.Resolve(l, args[1])
				if !ok {
					return fmt.Errorf("item not found: %s", args[1])
				}
				if it.InputValues == nil {
					it.InputValues = map[string]store.InputValue{}
				}
				// Clean up any prior secret/blob being replaced.
				if prev, ok := it.InputValues[args[2]]; ok {
					cleanupValue(prev, bs)
				}
				it.InputValues[args[2]] = newVal
				return nil
			})
			if err != nil {
				return fail("set", err)
			}
			return emit(map[string]string{"field": args[2]}, "set "+args[2])
		},
	}
	cmd.Flags().StringVar(&valueStr, "value", "", "literal value")
	cmd.Flags().StringVar(&filePath, "file", "", "read value from file (or upload, for file inputs)")
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "read value from stdin")
	return cmd
}

func readScalar(value string, fromStdin bool, filePath string) (string, error) {
	if fromStdin {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	if filePath != "" {
		b, err := os.ReadFile(filePath)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	return value, nil
}

func inputClearCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear <list> <item> <field>",
		Short: "Remove an input value (also purges secret/blob)",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			bs, _ := blobs.New(s.BlobsDir())
			err = s.Update(args[0], func(l *store.List) error {
				it, ok := tree.Resolve(l, args[1])
				if !ok {
					return fmt.Errorf("item not found: %s", args[1])
				}
				if prev, ok := it.InputValues[args[2]]; ok {
					cleanupValue(prev, bs)
					delete(it.InputValues, args[2])
				}
				return nil
			})
			if err != nil {
				return fail("clear", err)
			}
			return emit(map[string]string{"field": args[2]}, "cleared "+args[2])
		},
	}
	return cmd
}

// cleanupValue purges any external resource (secret/blob) held by v. Safe to
// call on values that have neither.
func cleanupValue(v store.InputValue, bs *blobs.Store) {
	if v.SecretRef != "" {
		if ref, err := secrets.ParseRef(v.SecretRef); err == nil {
			_ = secrets.Delete(ref)
		}
	}
	if v.BlobRef != "" && bs != nil {
		if sha, err := blobs.ParseRef(v.BlobRef); err == nil {
			_ = bs.Delete(sha)
		}
	}
}
