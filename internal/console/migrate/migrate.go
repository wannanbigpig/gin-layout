package migrate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/spf13/cobra"

	consolex "github.com/wannanbigpig/gin-layout/internal/console"
	"github.com/wannanbigpig/gin-layout/internal/service/system"
)

const (
	defaultMigrationDir    = "data/migrations"
	defaultMigrationExt    = "sql"
	defaultTimeFormat      = "20060102150405"
	defaultMigrationDigits = 6
)

var migrationFilePattern = regexp.MustCompile(`^(\d+)_(.+)\.(up|down)\.([^.]+)$`)

var (
	migratePath          string
	migrateAssumeYes     bool
	createUseSeq         bool
	createDigits         int
	createFormat         string
	createTZ             string
	createExt            string
	downAll              bool
	migrateCheckStrict   bool
	migrationNameCleaner = regexp.MustCompile(`[^a-z0-9_]+`)

	// Cmd 迁移命令入口。
	Cmd = &cobra.Command{
		Use:   "migrate",
		Short: "Database migration management commands (defaults to up)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUp(nil)
		},
	}

	createCmd = &cobra.Command{
		Use:   "create NAME",
		Short: "Create a migration pair (up/down)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(args[0])
		},
	}

	checkCmd = &cobra.Command{
		Use:   "check",
		Short: "Validate migration filename format and version pairing",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheck()
		},
	}

	upCmd = &cobra.Command{
		Use:   "up [N]",
		Short: "Apply all or N up migrations",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUp(args)
		},
	}

	downCmd = &cobra.Command{
		Use:   "down [N]",
		Short: "Apply 1, N, or all down migrations",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDown(args)
		},
	}

	gotoCmd = &cobra.Command{
		Use:   "goto VERSION",
		Short: "Migrate to a specific version",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			version, err := parseUint64Arg(args[0], "VERSION")
			if err != nil {
				return err
			}
			return runWithMigrator(func(m *migrate.Migrate) error {
				if err := m.Migrate(version); err != nil && !errors.Is(err, migrate.ErrNoChange) {
					return err
				}
				fmt.Printf("migrate goto %d complete\n", version)
				return nil
			})
		},
	}

	forceCmd = &cobra.Command{
		Use:   "force VERSION",
		Short: "Set migration version without running migrations",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			version, err := parseIntArg(args[0], "VERSION")
			if err != nil {
				return err
			}
			return runWithMigrator(func(m *migrate.Migrate) error {
				if err := m.Force(version); err != nil {
					return err
				}
				fmt.Printf("migrate force %d complete\n", version)
				return nil
			})
		},
	}

	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print current migration version",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithMigrator(func(m *migrate.Migrate) error {
				version, dirty, err := m.Version()
				if errors.Is(err, migrate.ErrNilVersion) {
					fmt.Println("version: none, dirty: false")
					return nil
				}
				if err != nil {
					return err
				}
				fmt.Printf("version: %d, dirty: %v\n", version, dirty)
				return nil
			})
		},
	}
)

func init() {
	registerFlags()
	registerSubCommands()
}

func registerFlags() {
	Cmd.PersistentFlags().StringVarP(&migratePath, "path", "p", "", "Migration directory path (default auto detect)")
	Cmd.PersistentFlags().BoolVarP(&migrateAssumeYes, "yes", "y", false, "Skip confirmation prompt")

	createCmd.Flags().BoolVar(&createUseSeq, "seq", false, "Generate sequential migration versions")
	createCmd.Flags().IntVar(&createDigits, "digits", defaultMigrationDigits, "Number of digits for sequential versions")
	createCmd.Flags().StringVar(&createFormat, "format", defaultTimeFormat, "Go time format for timestamp version")
	createCmd.Flags().StringVar(&createTZ, "tz", "UTC", "Timezone used for timestamp version")
	createCmd.Flags().StringVar(&createExt, "ext", defaultMigrationExt, "Migration file extension")

	downCmd.Flags().BoolVar(&downAll, "all", false, "Apply all down migrations")

	checkCmd.Flags().BoolVar(&migrateCheckStrict, "strict", true, "Fail if filename does not match migration pattern")
}

func registerSubCommands() {
	Cmd.AddCommand(createCmd)
	Cmd.AddCommand(checkCmd)
	Cmd.AddCommand(upCmd)
	Cmd.AddCommand(downCmd)
	Cmd.AddCommand(gotoCmd)
	Cmd.AddCommand(forceCmd)
	Cmd.AddCommand(versionCmd)
}

func runCreate(rawName string) error {
	dir, err := resolveMigrationDirForCreate()
	if err != nil {
		return err
	}

	name := normalizeMigrationName(rawName)
	if name == "" {
		return fmt.Errorf("migration name is empty after normalization")
	}

	ext := strings.TrimPrefix(strings.TrimSpace(createExt), ".")
	if ext == "" {
		ext = defaultMigrationExt
	}

	files, err := loadMigrationFiles(dir, migrateCheckStrict)
	if err != nil {
		return err
	}

	version, err := nextMigrationVersion(files)
	if err != nil {
		return err
	}

	upFile := filepath.Join(dir, fmt.Sprintf("%s_%s.up.%s", version, name, ext))
	downFile := filepath.Join(dir, fmt.Sprintf("%s_%s.down.%s", version, name, ext))
	if _, err := os.Stat(upFile); err == nil {
		return fmt.Errorf("target file already exists: %s", upFile)
	}
	if _, err := os.Stat(downFile); err == nil {
		return fmt.Errorf("target file already exists: %s", downFile)
	}

	if err := os.WriteFile(upFile, []byte("BEGIN;\n\n-- TODO: write migration up SQL.\n\nCOMMIT;\n"), 0o644); err != nil {
		return fmt.Errorf("write up migration failed: %w", err)
	}
	if err := os.WriteFile(downFile, []byte("BEGIN;\n\n-- TODO: write migration down SQL.\n\nCOMMIT;\n"), 0o644); err != nil {
		return fmt.Errorf("write down migration failed: %w", err)
	}

	fmt.Println(upFile)
	fmt.Println(downFile)
	return nil
}

func runCheck() error {
	dir, err := resolveMigrationDirForCheck()
	if err != nil {
		return err
	}

	files, err := loadMigrationFiles(dir, migrateCheckStrict)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no migration files found in %s", dir)
	}

	grouped := make(map[string]map[string]int, len(files))
	for _, file := range files {
		if _, ok := grouped[file.Version]; !ok {
			grouped[file.Version] = map[string]int{"up": 0, "down": 0}
		}
		grouped[file.Version][file.Direction]++
	}

	versions := make([]string, 0, len(grouped))
	for version := range grouped {
		versions = append(versions, version)
	}
	sort.Slice(versions, func(i, j int) bool {
		left, _ := strconv.ParseUint(versions[i], 10, 64)
		right, _ := strconv.ParseUint(versions[j], 10, 64)
		return left < right
	})

	for _, version := range versions {
		entry := grouped[version]
		if entry["up"] != 1 || entry["down"] != 1 {
			return fmt.Errorf("invalid version %s: up=%d down=%d (expect up=1 down=1)", version, entry["up"], entry["down"])
		}
	}

	fmt.Printf("[OK] migration check passed: %d versions, %d files.\n", len(versions), len(files))
	fmt.Printf("     first version: %s\n", versions[0])
	fmt.Printf("     last version:  %s\n", versions[len(versions)-1])
	return nil
}

func runUp(args []string) error {
	steps := 0
	if len(args) == 1 {
		n, err := parseIntArg(args[0], "N")
		if err != nil {
			return err
		}
		if n <= 0 {
			return fmt.Errorf("N must be greater than 0")
		}
		steps = n
	}

	return runWithMigrator(func(m *migrate.Migrate) error {
		if err := remapLegacySequentialVersionIfNeeded(m); err != nil {
			return err
		}

		var err error
		if steps > 0 {
			err = m.Steps(steps)
		} else {
			err = m.Up()
		}
		if err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return err
		}
		if steps > 0 {
			fmt.Printf("migrate up %d complete\n", steps)
		} else {
			fmt.Println("migrate up complete")
		}
		return nil
	})
}

func runDown(args []string) error {
	if downAll && len(args) > 0 {
		return fmt.Errorf("cannot use N with --all")
	}

	steps := 1
	if len(args) == 1 {
		n, err := parseIntArg(args[0], "N")
		if err != nil {
			return err
		}
		if n <= 0 {
			return fmt.Errorf("N must be greater than 0")
		}
		steps = n
	}

	if downAll {
		if !consolex.ConfirmOperation("This will apply all down migrations. Continue? [Y/N]: ", migrateAssumeYes) {
			fmt.Println("Operation cancelled.")
			return nil
		}
	}

	return runWithMigrator(func(m *migrate.Migrate) error {
		if err := remapLegacySequentialVersionIfNeeded(m); err != nil {
			return err
		}

		var err error
		if downAll {
			err = m.Down()
		} else {
			err = m.Steps(-steps)
		}
		if err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return err
		}
		if downAll {
			fmt.Println("migrate down --all complete")
		} else {
			fmt.Printf("migrate down %d complete\n", steps)
		}
		return nil
	})
}

func runWithMigrator(fn func(*migrate.Migrate) error) error {
	m, err := buildMigrator()
	if err != nil {
		return err
	}
	defer m.Close()

	return fn(m)
}

func buildMigrator() (*migrate.Migrate, error) {
	path := strings.TrimSpace(migratePath)
	if path == "" {
		return system.NewMigrator()
	}
	return system.NewMigratorWithPath(path)
}

func resolveMigrationDirForCreate() (string, error) {
	if strings.TrimSpace(migratePath) != "" {
		return ensureDirExists(migratePath)
	}
	if _, err := os.Stat(defaultMigrationDir); err == nil {
		return ensureDirExists(defaultMigrationDir)
	}
	return resolveMigrationDirForCheck()
}

func resolveMigrationDirForCheck() (string, error) {
	if strings.TrimSpace(migratePath) != "" {
		return ensureDirExists(migratePath)
	}
	path, err := system.ResolveMigrationsPath()
	if err != nil {
		return "", err
	}
	return ensureDirExists(path)
}

func ensureDirExists(path string) (string, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(path, "file://"))
	if trimmed == "" {
		return "", fmt.Errorf("migration path is empty")
	}
	absPath, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve migration path failed: %w", err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("migration path not found: %s", absPath)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("migration path is not a directory: %s", absPath)
	}
	return absPath, nil
}

func normalizeMigrationName(raw string) string {
	name := strings.ToLower(strings.TrimSpace(raw))
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, " ", "_")
	name = migrationNameCleaner.ReplaceAllString(name, "_")
	name = strings.Trim(name, "_")
	name = strings.ReplaceAll(name, "__", "_")
	return name
}

func nextMigrationVersion(files []migrationFile) (string, error) {
	if createUseSeq {
		if createDigits <= 0 {
			return "", fmt.Errorf("digits must be greater than 0")
		}
		maxVersion := 0
		for _, file := range files {
			v, err := strconv.Atoi(file.Version)
			if err != nil {
				continue
			}
			if v > maxVersion {
				maxVersion = v
			}
		}
		return fmt.Sprintf("%0*d", createDigits, maxVersion+1), nil
	}

	loc, err := time.LoadLocation(strings.TrimSpace(createTZ))
	if err != nil {
		return "", fmt.Errorf("invalid timezone: %w", err)
	}
	version := time.Now().In(loc).Format(createFormat)
	if _, err := strconv.ParseUint(version, 10, 64); err != nil {
		return "", fmt.Errorf("invalid time format result: %s (must be uint64)", version)
	}
	for _, file := range files {
		if file.Version == version {
			return "", fmt.Errorf("duplicate migration version %s, please retry", version)
		}
	}
	return version, nil
}

type migrationFile struct {
	Version   string
	Direction string
	Name      string
}

func loadMigrationFiles(dir string, strict bool) ([]migrationFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	result := make([]migrationFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		match := migrationFilePattern.FindStringSubmatch(name)
		if len(match) != 5 {
			if strict {
				return nil, fmt.Errorf("invalid migration filename format: %s", name)
			}
			continue
		}
		result = append(result, migrationFile{
			Version:   match[1],
			Direction: match[3],
			Name:      name,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Version == result[j].Version {
			return result[i].Name < result[j].Name
		}
		return result[i].Version < result[j].Version
	})
	return result, nil
}

func parseIntArg(value string, argName string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %s", argName, value)
	}
	return parsed, nil
}

func parseUint64Arg(value string, argName string) (uint, error) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %s", argName, value)
	}
	if parsed > uint64(^uint(0)) {
		return 0, fmt.Errorf("%s out of range: %s", argName, value)
	}
	return uint(parsed), nil
}

func remapLegacySequentialVersionIfNeeded(m *migrate.Migrate) error {
	currentVersion, dirty, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return nil
	}
	if err != nil || dirty {
		return nil
	}

	dir, err := resolveMigrationDirForCheck()
	if err != nil {
		return nil
	}
	files, err := loadMigrationFiles(dir, migrateCheckStrict)
	if err != nil || len(files) == 0 {
		return nil
	}

	versionSet := make(map[uint64]struct{}, len(files))
	uniqueVersions := make([]uint64, 0, len(files))
	allTimestampStyle := true
	for _, file := range files {
		if len(file.Version) < 10 {
			allTimestampStyle = false
		}
		v, parseErr := strconv.ParseUint(file.Version, 10, 64)
		if parseErr != nil {
			continue
		}
		if _, ok := versionSet[v]; ok {
			continue
		}
		versionSet[v] = struct{}{}
		uniqueVersions = append(uniqueVersions, v)
	}
	if len(uniqueVersions) == 0 || !allTimestampStyle {
		return nil
	}

	sort.Slice(uniqueVersions, func(i, j int) bool { return uniqueVersions[i] < uniqueVersions[j] })

	if _, ok := versionSet[uint64(currentVersion)]; ok {
		return nil
	}

	if currentVersion == 0 || int(currentVersion) > len(uniqueVersions) {
		return nil
	}

	target := uniqueVersions[int(currentVersion)-1]
	if target <= 999999999 {
		return nil
	}

	if err := m.Force(int(target)); err != nil {
		return fmt.Errorf("failed to remap legacy migration version %d to %d: %w", currentVersion, target, err)
	}
	fmt.Printf("detected legacy sequential migration version %d, remapped to timestamp version %d\n", currentVersion, target)
	return nil
}
