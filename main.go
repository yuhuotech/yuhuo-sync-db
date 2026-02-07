package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/yuhuo/sync-db/config"
	"github.com/yuhuo/sync-db/database"
	"github.com/yuhuo/sync-db/logger"
	"github.com/yuhuo/sync-db/sync"
	"github.com/yuhuo/sync-db/ui"
)

func main() {
	configFile := flag.String("config", "config.yaml", "Path to config file")
	flag.Parse()

	// 加载配置
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	appLogger, err := logger.NewLogger(cfg.Logging.Level, cfg.Logging.File)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer appLogger.Close()

	appLogger.Info("Application started")
	appLogger.Info(fmt.Sprintf("Connecting to source database: %s:%d/%s", cfg.Source.Host, cfg.Source.Port, cfg.Source.Database))
	appLogger.Info(fmt.Sprintf("Connecting to target database: %s:%d/%s", cfg.Target.Host, cfg.Target.Port, cfg.Target.Database))

	// 连接数据库
	connManager, err := database.NewConnectionManager(&cfg.Source, &cfg.Target)
	if err != nil {
		appLogger.Error(fmt.Sprintf("Failed to connect to databases: %v", err))
		fmt.Fprintf(os.Stderr, "Failed to connect to databases: %v\n", err)
		os.Exit(1)
	}
	defer connManager.Close()

	appLogger.Info("Successfully connected to both databases")

	// 第一步：比对差异
	fmt.Println("\n========== Step 1: Comparing Differences ==========\n")
	appLogger.Info("Starting difference comparison")

	comparator := sync.NewComparator(connManager.GetSourceDB(), connManager.GetTargetDB())
	diff, err := comparator.CompareDifferences(cfg.SyncDataTables)
	if err != nil {
		appLogger.Error(fmt.Sprintf("Failed to compare differences: %v", err))
		fmt.Fprintf(os.Stderr, "Failed to compare differences: %v\n", err)
		os.Exit(1)
	}

	appLogger.Info(fmt.Sprintf("Comparison complete: %d structure diffs, %d data diffs, %d view diffs",
		len(diff.StructureDifferences), len(diff.DataDifferences), len(diff.ViewDifferences)))

	// 展示差异
	ui.PrintDifferenceSummary(diff)

	if !diff.HasDifferences() {
		fmt.Println("✓ No differences found. Sync is already complete!")
		appLogger.Info("No differences found")
		return
	}

	// 用户确认
	if !ui.ConfirmContinue("Do you want to continue with the sync?") {
		fmt.Println("Sync cancelled by user")
		appLogger.Info("Sync cancelled by user")
		return
	}

	// 第二步：生成 SQL
	fmt.Println("\n========== Step 2: Generating SQL Statements ==========\n")
	appLogger.Info("Generating SQL statements")

	sqlGen := sync.NewSQLGenerator(connManager.GetSourceDB())
	sqls, err := sqlGen.GenerateSQL(diff)
	if err != nil {
		appLogger.Error(fmt.Sprintf("Failed to generate SQL: %v", err))
		fmt.Fprintf(os.Stderr, "Failed to generate SQL: %v\n", err)
		os.Exit(1)
	}

	appLogger.Info(fmt.Sprintf("Generated %d SQL statements", len(sqls)))

	// 展示 SQL
	ui.PrintSQLStatements(sqls)

	// 用户确认执行
	if !ui.ConfirmContinue("Do you want to execute these SQL statements?") {
		fmt.Println("SQL execution cancelled by user")
		appLogger.Info("SQL execution cancelled by user")
		return
	}

	// 第三步：执行 SQL
	fmt.Println("\n========== Step 3: Executing SQL Statements ==========\n")
	appLogger.Info("Starting SQL execution")

	executor := sync.NewExecutor(connManager.GetTargetDB(), appLogger)
	results := executor.ExecuteSQL(sqls)

	total, success, failed := sync.GetSummary(results)
	appLogger.Info(fmt.Sprintf("SQL execution complete: %d total, %d success, %d failed", total, success, failed))

	ui.PrintExecutionSummary(total, success, failed)

	if failed > 0 {
		failedResults := sync.GetFailedResults(results)
		fmt.Println("Failed SQL statements:")
		for _, result := range failedResults {
			fmt.Printf("  SQL: %s\n", result.SQL)
			fmt.Printf("  Error: %v\n\n", result.Error)
			appLogger.Error(fmt.Sprintf("Failed SQL: %s, Error: %v", result.SQL, result.Error))
		}
	}

	// 第四步：验证
	fmt.Println("\n========== Step 4: Verifying Sync Results ==========\n")
	appLogger.Info("Starting verification")

	verifier := sync.NewVerifier(connManager.GetSourceDB(), connManager.GetTargetDB())
	verifySuccess, verifyMessage, err := verifier.VerifySync(cfg.SyncDataTables)
	if err != nil {
		appLogger.Error(fmt.Sprintf("Failed to verify sync: %v", err))
		fmt.Fprintf(os.Stderr, "Failed to verify sync: %v\n", err)
		os.Exit(1)
	}

	appLogger.Info(verifyMessage)
	ui.PrintVerificationResult(verifySuccess, verifyMessage)

	if verifySuccess {
		fmt.Println("✓ All steps completed successfully!")
		appLogger.Info("Sync completed successfully")
	} else {
		fmt.Println("✗ Sync verification failed! Please check the errors above.")
		appLogger.Error("Sync verification failed")
	}
}
