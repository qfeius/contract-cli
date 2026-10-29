package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"cn.qfei/contract-cli/internal/build"
	"cn.qfei/contract-cli/internal/skillsync"
)

func (a *App) printSkillCLIIdentity() {
	executable, err := os.Executable()
	if err != nil {
		_, _ = fmt.Fprintln(a.stderr, "CLI 实际可执行文件：无法获取；不能据此推断 PATH 或其他 CLI 副本。")
		return
	}
	_, _ = fmt.Fprintf(a.stdout, "CLI 版本：%s\nCLI 实际可执行文件：%s\n", build.Current().Version, executable)
}

func (a *App) printRegisteredSkillLocations(service *skillsync.Service) {
	_, _ = fmt.Fprintf(a.stdout, "Skill 登记文件：%s\n", filepath.Join(service.StateDir, "skill-locations.json"))
	locations, err := service.RegisteredLocations()
	if err != nil {
		_, _ = fmt.Fprintf(a.stderr, "已登记的 Skill 目录：读取失败；失败原因：%v。不能据此认为登记为空。\n", err)
		return
	}
	_, _ = fmt.Fprintf(a.stdout, "已登记的 Skill 目录：%d（登记记录不等于当前客户端已加载，也不保证目录仍有效）\n", len(locations))
	for _, location := range locations {
		_, _ = fmt.Fprintf(a.stdout, "  %s；记录版本：%s；记录状态：%s\n", location.Target, location.Version, location.Status)
		if location.Error != "" {
			_, _ = fmt.Fprintf(a.stdout, "    最近记录原因：%s\n", location.Error)
		}
	}
	_, _ = fmt.Fprintln(a.stdout, "文件更新不代表客户端已加载；请刷新、重启或新建客户端会话验证。")
}

func recoveryDescription(status string) string {
	switch status {
	case "committed":
		return "取消到达前安装已提交；已保留安装文件及登记，未执行回滚"
	case "restored":
		return "已恢复本次替换前的原有文件；未保留长期历史备份"
	case "failed":
		return "恢复失败；请保留事务恢复材料，不要直接清理或覆盖"
	default:
		return "无法确认（未获得恢复成功证据；可能未执行替换，不代表旧文件已恢复）"
	}
}

func (a *App) printSkillFailure(operation, target, reason, recovery string) {
	_, _ = fmt.Fprintf(a.stderr, "%s：失败\nSkill 安装位置：%s\n失败原因：%s\n恢复结果：%s\n", operation, target, reason, recoveryDescription(recovery))
	_, _ = fmt.Fprintln(a.stderr, "处理建议："+skillFailureGuidance(target, recovery))
}

func skillFailureGuidance(target, recovery string) string {
	switch {
	case recovery == "committed":
		return "先核实已提交的安装文件和登记；不要把取消当作安装未发生，也不要直接重复覆盖。"
	case recovery == "failed":
		return "先排查恢复失败原因并核实原文件和事务材料，不能假定文件已恢复。"
	case target == "":
		return "先核实参数、登记文件及其权限，不能把本次失败当成没有历史安装。"
	case skillsync.IsClientManagedSkillTarget(runtime.GOOS, target):
		return "不要向 SYSTEM 目录重新安装；确认客户端用户 Skill 目录并核实旧登记。"
	default:
		return fmt.Sprintf("先排查失败原因，核实目录、权限和备份；修复并确认后执行：%s", skillInstallCommand(target))
	}
}
