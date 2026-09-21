package build

import "fmt"

const Name = "contract-cli"

const (
	// FeatureApprovalMatrixExtensions 是审批矩阵扩展命令集的稳定标识。
	FeatureApprovalMatrixExtensions = "approval-matrix-extensions"
	// FeatureApprovalMatrixExtensionsSinceVersion 记录扩展命令首次进入可交付构建的版本。
	FeatureApprovalMatrixExtensionsSinceVersion = "1.8.3-test.13"
	// FeatureApprovalMatrixExtensionsCommit 记录扩展命令首次合入的提交短 hash。
	FeatureApprovalMatrixExtensionsCommit = "61d8aa6"
)

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

type Info struct {
	Name    string
	Version string
	Commit  string
	Date    string
}

func Current() Info {
	return Info{
		Name:    Name,
		Version: defaultString(Version, "dev"),
		Commit:  defaultString(Commit, "unknown"),
		Date:    defaultString(Date, "unknown"),
	}
}

/*
FeatureBaseline 返回当前源码包含的功能基线说明。
无入参；返回值 string 为可直接展示给 CLI 使用者的功能基线文本。
*/
func FeatureBaseline() string {
	return fmt.Sprintf("feature baseline %s since %s (commit %s)", FeatureApprovalMatrixExtensions, FeatureApprovalMatrixExtensionsSinceVersion, FeatureApprovalMatrixExtensionsCommit)
}

/*
String 格式化版本、提交、构建时间和功能基线。
入参 i（Info）为待格式化的构建元信息；返回值 string 为版本命令展示文本。
*/
func (i Info) String() string {
	return fmt.Sprintf("%s version %s (commit %s, built %s; %s)", i.Name, defaultString(i.Version, "dev"), defaultString(i.Commit, "unknown"), defaultString(i.Date, "unknown"), FeatureBaseline())
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
