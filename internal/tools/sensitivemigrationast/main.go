package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strconv"
	"strings"
)

type analysis struct {
	DriverGateExact           bool                `json:"driverGateExact"`
	PrepareOwnsJournalSchema  bool                `json:"prepareOwnsJournalSchema"`
	ReadOnlyDispatch          bool                `json:"readOnlyDispatch"`
	PreparedDispatch          bool                `json:"preparedDispatch"`
	VerifyPathSafe            bool                `json:"verifyPathSafe"`
	VerifyStateLoaderReadOnly bool                `json:"verifyStateLoaderReadOnly"`
	PlaintextRejected         bool                `json:"plaintextRejected"`
	APIStartupSafe            bool                `json:"apiStartupSafe"`
	CLIReportBeforeClose      bool                `json:"cliReportBeforeClose"`
	CLIErrorsValueFree        bool                `json:"cliErrorsValueFree"`
	Fingerprints              []sourceFingerprint `json:"fingerprints"`
}

type parsedSource struct {
	file       *ast.File
	fileSet    *token.FileSet
	directives []string
}

type sourceFingerprint struct {
	Path        string `json:"path"`
	Role        string `json:"role"`
	Scope       string `json:"scope"`
	Symbol      string `json:"symbol"`
	Fingerprint string `json:"fingerprint"`
}

var mutationOrDecryptionSelectors = []string{
	"Prepare", "ApplyBatch", "FinishRun", "RollbackBatch", "CommitRehearsal", "FinishRollback", "AutoMigrate", "Protect", "Reveal",
}

func main() {
	bootstrapPath := flag.String("bootstrap", "", "migration bootstrap source")
	runnerPath := flag.String("runner", "", "migration runner source")
	gormStorePath := flag.String("gorm-store", "", "migration GORM store source")
	protectionPath := flag.String("protection-source", "", "ordinary protected store source")
	apiMainPath := flag.String("api-main", "", "API composition root source")
	cliPath := flag.String("cli", "", "maintenance CLI source")
	flag.Parse()

	files := make(map[string]parsedSource, 6)
	for label, filePath := range map[string]string{
		"bootstrap":         *bootstrapPath,
		"runner":            *runnerPath,
		"gorm store":        *gormStorePath,
		"protection source": *protectionPath,
		"API main":          *apiMainPath,
		"CLI":               *cliPath,
	} {
		source, err := parseFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", label, err)
			os.Exit(2)
		}
		files[label] = source
	}
	fingerprints, err := sourceFingerprints(files)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fingerprint migration source AST")
		os.Exit(2)
	}

	result := analysis{
		DriverGateExact:          exactDriverGate(files["bootstrap"].file),
		PrepareOwnsJournalSchema: prepareOwnsAutoMigrate(files["gorm store"].file),
		ReadOnlyDispatch:         exactModeDispatch(files["runner"].file, "Run", []string{"ModeInventory", "ModeDryRun"}, "runReadOnly"),
		PreparedDispatch:         exactModeDispatch(files["runner"].file, "Run", []string{"ModePrepare", "ModeApply", "ModeVerify", "ModeRehearseRestore", "ModeRollback"}, "runPrepared"),
		VerifyPathSafe:           verifyPathSafe(files["runner"].file),
		VerifyStateLoaderReadOnly: functionAvoidsSelectors(
			files["gorm store"].file, "StartOrResume", "GORMProtectedValueMigrationStore",
			forbiddenSelectors("Create", "Delete", "Exec", "Save", "Update", "Updates"),
		),
		PlaintextRejected:    plaintextRejected(files["protection source"].file),
		APIStartupSafe:       functionAvoidsSelectors(files["API main"].file, "main", "", []string{"OpenSensitiveDataMigration", "NewRunner"}),
		CLIReportBeforeClose: cliReportBeforeClose(files["CLI"].file),
		CLIErrorsValueFree:   cliErrorsValueFree(files["CLI"].file),
		Fingerprints:         fingerprints,
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, "encode AST analysis")
		os.Exit(2)
	}
}

func parseFile(filePath string) (parsedSource, error) {
	if strings.TrimSpace(filePath) == "" {
		return parsedSource{}, fmt.Errorf("source path is required")
	}
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, filePath, nil, parser.SkipObjectResolution)
	if err != nil {
		return parsedSource{}, err
	}
	commented, err := parser.ParseFile(token.NewFileSet(), filePath, nil, parser.SkipObjectResolution|parser.ParseComments)
	if err != nil {
		return parsedSource{}, err
	}
	return parsedSource{file: file, fileSet: fileSet, directives: semanticDirectives(commented)}, nil
}

func semanticDirectives(file *ast.File) []string {
	if file == nil {
		return nil
	}
	directives := make([]string, 0)
	for _, group := range file.Comments {
		for _, comment := range group.List {
			text := strings.TrimSpace(comment.Text)
			if strings.HasPrefix(text, "//go:") {
				directives = append(directives, text)
				continue
			}
			body := strings.TrimSpace(strings.TrimPrefix(text, "//"))
			if body == "+build" {
				directives = append(directives, "// +build")
			} else if strings.HasPrefix(body, "+build ") || strings.HasPrefix(body, "+build\t") {
				directives = append(directives, "// +build "+strings.TrimSpace(body[len("+build"):]))
			}
		}
	}
	return directives
}

func sourceFingerprints(files map[string]parsedSource) ([]sourceFingerprint, error) {
	type specification struct {
		label    string
		path     string
		role     string
		scope    string
		symbol   string
		function string
		receiver string
	}
	specifications := []specification{
		{label: "runner", path: "internal/platform/sensitivemigration/runner.go", role: "migration-runner", scope: "file"},
		{label: "gorm store", path: "internal/platform/adminresource/sensitive_migration_gorm.go", role: "migration-gorm-store", scope: "file"},
		{label: "bootstrap", path: "internal/platform/bootstrap/sensitive_migration.go", role: "migration-bootstrap", scope: "file"},
		{label: "protection source", path: "internal/platform/adminresource/security.go", role: "ordinary-store-plaintext-rejection", scope: "function", symbol: "(*Store).validateProtectedRecord", function: "validateProtectedRecord", receiver: "Store"},
		{label: "CLI", path: "cmd/platform-admin/main.go", role: "maintenance-cli", scope: "function", symbol: "runSensitiveDataMigration", function: "runSensitiveDataMigration"},
		{label: "API main", path: "cmd/platform-api/main.go", role: "api-startup", scope: "function", symbol: "main", function: "main"},
	}
	result := make([]sourceFingerprint, 0, len(specifications))
	for _, specification := range specifications {
		source, ok := files[specification.label]
		if !ok || source.file == nil || source.fileSet == nil {
			return nil, fmt.Errorf("missing parsed source")
		}
		var node ast.Node = source.file
		if specification.scope == "function" {
			node = function(source.file, specification.function, specification.receiver)
			if node == nil {
				return nil, fmt.Errorf("missing protected function")
			}
		}
		fingerprint, err := canonicalFingerprint(source.fileSet, node, source.directives)
		if err != nil {
			return nil, err
		}
		result = append(result, sourceFingerprint{
			Path: specification.path, Role: specification.role, Scope: specification.scope,
			Symbol: specification.symbol, Fingerprint: fingerprint,
		})
	}
	return result, nil
}

func canonicalFingerprint(fileSet *token.FileSet, node ast.Node, directives []string) (string, error) {
	var canonical bytes.Buffer
	if len(directives) > 0 {
		encoded, err := json.Marshal(directives)
		if err != nil {
			return "", err
		}
		canonical.WriteString("semantic-directives:")
		canonical.Write(encoded)
		canonical.WriteByte('\n')
	}
	if err := format.Node(&canonical, fileSet, node); err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical.Bytes())
	return fmt.Sprintf("sha256:%x", digest), nil
}

func function(file *ast.File, name string, receiver string) *ast.FuncDecl {
	var matched *ast.FuncDecl
	for _, declaration := range file.Decls {
		candidate, ok := declaration.(*ast.FuncDecl)
		if !ok || candidate.Name.Name != name || receiverType(candidate) != receiver {
			continue
		}
		if matched != nil {
			return nil
		}
		matched = candidate
	}
	return matched
}

func receiverType(fn *ast.FuncDecl) string {
	if fn == nil || fn.Recv == nil || len(fn.Recv.List) != 1 {
		return ""
	}
	receiver := fn.Recv.List[0].Type
	if pointer, ok := receiver.(*ast.StarExpr); ok {
		receiver = pointer.X
	}
	identifier, _ := receiver.(*ast.Ident)
	if identifier == nil {
		return ""
	}
	return identifier.Name
}

func exactDriverGate(file *ast.File) bool {
	fn := function(file, "sensitiveMigrationGORMDriver", "")
	if fn == nil || fn.Body == nil || len(fn.Body.List) != 1 {
		return false
	}
	switchStatement, ok := fn.Body.List[0].(*ast.SwitchStmt)
	driver, directDriver := switchStatement.Tag.(*ast.Ident)
	if !ok || switchStatement.Init != nil || !directDriver || driver.Name != "driver" || len(switchStatement.Body.List) != 2 {
		return false
	}

	allowed := []string{"mysql", "postgres", "sqlite"}
	matchedAllowed := false
	matchedDefault := false
	for _, statement := range switchStatement.Body.List {
		clause, ok := statement.(*ast.CaseClause)
		if !ok || len(clause.Body) != 1 {
			return false
		}
		if clause.List == nil {
			matchedDefault = returnsBoolean(clause.Body[0], false)
			continue
		}
		values := make([]string, 0, len(clause.List))
		for _, expression := range clause.List {
			literal, ok := expression.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return false
			}
			value, err := strconv.Unquote(literal.Value)
			if err != nil {
				return false
			}
			values = append(values, value)
		}
		slices.Sort(values)
		matchedAllowed = slices.Equal(values, allowed) && returnsBoolean(clause.Body[0], true)
	}
	return matchedAllowed && matchedDefault
}

func returnsBoolean(statement ast.Stmt, expected bool) bool {
	result, ok := statement.(*ast.ReturnStmt)
	if !ok || len(result.Results) != 1 {
		return false
	}
	identifier, ok := result.Results[0].(*ast.Ident)
	return ok && identifier.Name == strconv.FormatBool(expected)
}

func prepareOwnsAutoMigrate(file *ast.File) bool {
	owners := 0
	for _, declaration := range file.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok || !containsAnySelector(fn, []string{"AutoMigrate"}) {
			continue
		}
		if fn.Name.Name != "Prepare" || receiverType(fn) != "GORMProtectedValueMigrationStore" {
			return false
		}
		owners++
	}
	return owners == 1
}

func exactModeDispatch(file *ast.File, functionName string, modes []string, targetCall string) bool {
	fn := function(file, functionName, "Runner")
	if fn == nil {
		return false
	}
	for _, statement := range fn.Body.List {
		switchStatement, ok := statement.(*ast.SwitchStmt)
		if !ok || !selectorExpression(switchStatement.Tag, "options", "Mode") {
			continue
		}
		for _, switchBody := range switchStatement.Body.List {
			clause, ok := switchBody.(*ast.CaseClause)
			if ok && sameCaseNames(clause, modes) && len(clause.Body) == 1 && directReturnCall(clause.Body, "r", targetCall) {
				return true
			}
		}
	}
	return false
}

func verifyPathSafe(file *ast.File) bool {
	prepared := function(file, "runPrepared", "Runner")
	verify := function(file, "runVerify", "Runner")
	if prepared == nil || verify == nil {
		return false
	}
	forbiddenDispatch := mutationOrDecryptionSelectors
	forbiddenVerify := forbiddenSelectors("StartOrResume")
	if containsAnySelector(verify, forbiddenVerify) {
		return false
	}
	for _, statement := range prepared.Body.List {
		switchStatement, ok := statement.(*ast.SwitchStmt)
		if !ok || !selectorExpression(switchStatement.Tag, "options", "Mode") {
			continue
		}
		for _, switchBody := range switchStatement.Body.List {
			clause, ok := switchBody.(*ast.CaseClause)
			if !ok || !sameCaseNames(clause, []string{"ModeVerify"}) {
				continue
			}
			return directAssignmentCall(clause.Body, "store", "StartOrResume") && directReturnCall(clause.Body, "r", "runVerify") &&
				!containsAnySelector(clause, forbiddenDispatch)
		}
	}
	return false
}

func forbiddenSelectors(additional ...string) []string {
	result := make([]string, 0, len(mutationOrDecryptionSelectors)+len(additional))
	result = append(result, mutationOrDecryptionSelectors...)
	return append(result, additional...)
}

func functionAvoidsSelectors(file *ast.File, functionName string, receiver string, forbidden []string) bool {
	fn := function(file, functionName, receiver)
	return fn != nil && !containsAnySelector(fn, forbidden)
}

func plaintextRejected(file *ast.File) bool {
	fn := function(file, "validateProtectedRecord", "Store")
	if fn == nil || containsAnySelector(fn, []string{"Protect", "Reveal"}) {
		return false
	}
	for _, statement := range fn.Body.List {
		fieldLoop, ok := statement.(*ast.RangeStmt)
		if !ok || !selectorExpression(fieldLoop.X, "schema", "Fields") {
			continue
		}
		for _, fieldStatement := range fieldLoop.Body.List {
			rejection, ok := fieldStatement.(*ast.IfStmt)
			if !ok || !negatedCall(rejection.Cond, "dataprotection", "IsEnvelope", "envelope") || len(rejection.Body.List) != 1 {
				continue
			}
			result, ok := rejection.Body.List[0].(*ast.ReturnStmt)
			return ok && len(result.Results) == 1 && callName(result.Results[0]) == "invalidSecurityField"
		}
	}
	return false
}

func negatedCall(expression ast.Expr, receiver string, name string, argument string) bool {
	unary, ok := expression.(*ast.UnaryExpr)
	if !ok || unary.Op != token.NOT {
		return false
	}
	call, ok := unary.X.(*ast.CallExpr)
	return ok && selectorCall(call, receiver, name) && len(call.Args) == 1 && identifierName(call.Args[0]) == argument
}

func cliReportBeforeClose(file *ast.File) bool {
	fn := function(file, "runSensitiveDataMigration", "")
	if fn == nil || countReportEncodes(fn) != 1 || countSelectorReferences(fn, "session", "Close") != 2 {
		return false
	}
	encodeIndex := -1
	closeIndex := -1
	deferIndex := -1
	for index, statement := range fn.Body.List {
		deferredClose := topLevelDeferredSessionClose(statement)
		explicitClose := topLevelSessionClose(statement)
		if countSelectorReferences(statement, "session", "Close") > 0 && !deferredClose && !explicitClose {
			return false
		}
		if topLevelReportEncode(statement) {
			if encodeIndex >= 0 {
				return false
			}
			encodeIndex = index
		}
		if explicitClose {
			if closeIndex >= 0 {
				return false
			}
			closeIndex = index
		}
		if deferredClose {
			if deferIndex >= 0 {
				return false
			}
			deferIndex = index
		}
	}
	return deferIndex >= 0 && deferIndex < encodeIndex && closeIndex > encodeIndex
}

func topLevelReportEncode(statement ast.Stmt) bool {
	condition, ok := statement.(*ast.IfStmt)
	if !ok || condition.Else != nil || len(condition.Body.List) != 1 {
		return false
	}
	assignment, ok := condition.Init.(*ast.AssignStmt)
	if !ok || assignment.Tok != token.DEFINE || len(assignment.Lhs) != 1 || identifierName(assignment.Lhs[0]) != "err" || len(assignment.Rhs) != 1 {
		return false
	}
	call, ok := assignment.Rhs[0].(*ast.CallExpr)
	if !ok || !encodedReportCall(call) {
		return false
	}
	comparison, ok := condition.Cond.(*ast.BinaryExpr)
	if !ok || comparison.Op != token.NEQ || identifierName(comparison.X) != "err" || identifierName(comparison.Y) != "nil" {
		return false
	}
	result, ok := condition.Body.List[0].(*ast.ReturnStmt)
	return ok && len(result.Results) == 1 && fixedCLIError(result.Results[0])
}

func topLevelSessionClose(statement ast.Stmt) bool {
	assignment, ok := statement.(*ast.AssignStmt)
	if !ok || assignment.Tok != token.DEFINE || len(assignment.Lhs) != 1 || identifierName(assignment.Lhs[0]) != "closeErr" || len(assignment.Rhs) != 1 {
		return false
	}
	call, ok := assignment.Rhs[0].(*ast.CallExpr)
	return ok && selectorCall(call, "session", "Close") && len(call.Args) == 0
}

func topLevelDeferredSessionClose(statement ast.Stmt) bool {
	deferred, ok := statement.(*ast.DeferStmt)
	if !ok || deferred.Call == nil || len(deferred.Call.Args) != 0 {
		return false
	}
	cleanup, ok := deferred.Call.Fun.(*ast.FuncLit)
	if !ok || cleanup.Type.Params == nil || len(cleanup.Type.Params.List) != 0 || len(cleanup.Body.List) != 1 {
		return false
	}
	guard, ok := cleanup.Body.List[0].(*ast.IfStmt)
	if !ok || guard.Init != nil || guard.Else != nil || !negatedIdentifier(guard.Cond, "closed") || len(guard.Body.List) != 1 {
		return false
	}
	assignment, ok := guard.Body.List[0].(*ast.AssignStmt)
	if !ok || assignment.Tok != token.ASSIGN || len(assignment.Lhs) != 1 || identifierName(assignment.Lhs[0]) != "_" || len(assignment.Rhs) != 1 {
		return false
	}
	call, ok := assignment.Rhs[0].(*ast.CallExpr)
	return ok && selectorCall(call, "session", "Close") && len(call.Args) == 0
}

func negatedIdentifier(expression ast.Expr, name string) bool {
	unary, ok := expression.(*ast.UnaryExpr)
	return ok && unary.Op == token.NOT && identifierName(unary.X) == name
}

func countReportEncodes(node ast.Node) int {
	count := 0
	ast.Inspect(node, func(candidate ast.Node) bool {
		call, ok := candidate.(*ast.CallExpr)
		if ok && encodedReportCall(call) {
			count++
		}
		return true
	})
	return count
}

func countSelectorReferences(node ast.Node, receiver string, name string) int {
	count := 0
	ast.Inspect(node, func(candidate ast.Node) bool {
		selector, ok := candidate.(*ast.SelectorExpr)
		if ok && selectorExpression(selector, receiver, name) {
			count++
		}
		return true
	})
	return count
}

func encodedReportCall(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Encode" || len(call.Args) != 1 || identifierName(call.Args[0]) != "report" {
		return false
	}
	encoder, ok := selector.X.(*ast.CallExpr)
	return ok && selectorCall(encoder, "json", "NewEncoder") && len(encoder.Args) == 1 && identifierName(encoder.Args[0]) == "stdout"
}

func cliErrorsValueFree(file *ast.File) bool {
	fn := function(file, "runSensitiveDataMigration", "")
	if fn == nil {
		return false
	}
	valueFree := true
	ast.Inspect(fn, func(node ast.Node) bool {
		result, ok := node.(*ast.ReturnStmt)
		if ok && (len(result.Results) != 1 || !fixedCLIError(result.Results[0])) {
			valueFree = false
			return false
		}
		return true
	})
	return valueFree
}

func fixedCLIError(expression ast.Expr) bool {
	if identifierName(expression) == "nil" {
		return true
	}
	call, ok := expression.(*ast.CallExpr)
	if !ok || !selectorCall(call, "errors", "New") || len(call.Args) != 1 {
		return false
	}
	literal, ok := call.Args[0].(*ast.BasicLit)
	return ok && literal.Kind == token.STRING
}

func sameCaseNames(clause *ast.CaseClause, expected []string) bool {
	actual := make([]string, 0, len(clause.List))
	for _, expression := range clause.List {
		name := identifierName(expression)
		if name == "" {
			return false
		}
		actual = append(actual, name)
	}
	slices.Sort(actual)
	want := append([]string(nil), expected...)
	slices.Sort(want)
	return slices.Equal(actual, want)
}

func selectorExpression(expression ast.Expr, receiver string, name string) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	return ok && identifierName(selector.X) == receiver && selector.Sel.Name == name
}

func selectorCall(call *ast.CallExpr, receiver string, name string) bool {
	return call != nil && selectorExpression(call.Fun, receiver, name)
}

func directReturnCall(statements []ast.Stmt, receiver string, name string) bool {
	matches := 0
	for _, statement := range statements {
		result, ok := statement.(*ast.ReturnStmt)
		if !ok || len(result.Results) != 1 {
			continue
		}
		call, ok := result.Results[0].(*ast.CallExpr)
		if ok && selectorCall(call, receiver, name) {
			matches++
		}
	}
	return matches == 1
}

func directAssignmentCall(statements []ast.Stmt, receiver string, name string) bool {
	matches := 0
	for _, statement := range statements {
		assignment, ok := statement.(*ast.AssignStmt)
		if !ok {
			continue
		}
		for _, expression := range assignment.Rhs {
			call, ok := expression.(*ast.CallExpr)
			if ok && selectorCall(call, receiver, name) {
				matches++
			}
		}
	}
	return matches == 1
}

func containsAnySelector(node ast.Node, names []string) bool {
	forbidden := make(map[string]struct{}, len(names))
	for _, name := range names {
		forbidden[name] = struct{}{}
	}
	found := false
	ast.Inspect(node, func(candidate ast.Node) bool {
		selector, ok := candidate.(*ast.SelectorExpr)
		if ok {
			if _, rejected := forbidden[selector.Sel.Name]; rejected {
				found = true
			}
		}
		return !found
	})
	return found
}

func callName(expression ast.Expr) string {
	call, ok := expression.(*ast.CallExpr)
	if !ok {
		return ""
	}
	switch function := call.Fun.(type) {
	case *ast.Ident:
		return function.Name
	case *ast.SelectorExpr:
		return function.Sel.Name
	default:
		return ""
	}
}

func identifierName(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		return value.Sel.Name
	default:
		return ""
	}
}
