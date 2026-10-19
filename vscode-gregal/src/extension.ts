import * as vscode from "vscode";
import { apiPost, listCheckpoints, rewindAll, rewindTo } from "./api";
import { activateChat, deactivateChat, connectionConfig, withIdleConversation } from "./controller";

async function rewindCmd(): Promise<void> {
    // Paritat amb el botó Desfés del web: tria checkpoint o desfés-ho tot.
    // Restaura el contingut d'abans de la sessió (no usa git).
    const { base, token, session } = await connectionConfig();
    let cps;
    try {
        cps = await listCheckpoints(base, token, session);
    } catch (e) {
        void vscode.window.showErrorMessage(vscode.l10n.t("Gregal: could not read checkpoints: {0}", e instanceof Error ? e.message : String(e)));
        return;
    }
    if (!cps.length) {
        void vscode.window.showInformationMessage(vscode.l10n.t("Gregal: nothing to undo (no files changed this session)."));
        return;
    }
    const items = cps.map((c) => ({
        label: `#${c.seq} ${c.op} ${c.path}`,
        description: c.at,
        seq: c.seq,
    }));
    const countDescription = cps.length === 1
        ? vscode.l10n.t("1 change")
        : vscode.l10n.t("{0} changes", cps.length);
    const pick = await vscode.window.showQuickPick(
        [{ label: vscode.l10n.t("↩ Undo all changes"), description: countDescription, seq: 0 }, ...items],
        { placeHolder: vscode.l10n.t("Return to a checkpoint (restores files; does not use Git)") },
    );
    if (!pick) return;
    const ok = await vscode.window.showWarningMessage(
        pick.seq === 0
            ? cps.length === 1
                ? vscode.l10n.t("Undo this session change?")
                : vscode.l10n.t("Undo all {0} session changes?", cps.length)
            : vscode.l10n.t("Return to checkpoint #{0}?", pick.seq),
        { modal: true },
        vscode.l10n.t("Undo"),
    );
    if (ok !== vscode.l10n.t("Undo")) return;
    try {
        const r = pick.seq === 0 ? await rewindAll(base, token, session) : await rewindTo(base, token, pick.seq, session);
        void vscode.window.showInformationMessage("Gregal: " + (r.summary || vscode.l10n.t("changes undone")));
    } catch (e) {
        void vscode.window.showErrorMessage(vscode.l10n.t("Gregal: could not undo: {0}", e instanceof Error ? e.message : String(e)));
    }
}

async function githubCmd(): Promise<void> {
    const kind = await vscode.window.showQuickPick(
        [{ label: vscode.l10n.t("Issue"), tool: "gh_issue" }, { label: vscode.l10n.t("Pull request"), tool: "gh_pr" }],
        { placeHolder: vscode.l10n.t("What would you like to view on GitHub?") },
    );
    if (!kind) return;
    const actions = kind.tool === "gh_issue"
        ? [{ label: vscode.l10n.t("List"), action: "list" }, { label: vscode.l10n.t("View issue"), action: "view" }]
        : [{ label: vscode.l10n.t("List"), action: "list" }, { label: vscode.l10n.t("View pull request"), action: "view" }, { label: vscode.l10n.t("Diff"), action: "diff" }, { label: vscode.l10n.t("Checks"), action: "checks" }];
    const selected = await vscode.window.showQuickPick(actions, { placeHolder: vscode.l10n.t("Read-only action") });
    if (!selected) return;
    const number = selected.action === "list" ? "" : await vscode.window.showInputBox({ prompt: vscode.l10n.t("Number (leave empty for the current repository)") });
    if (selected.action !== "list" && number && !/^\d+$/.test(number.trim())) {
        void vscode.window.showErrorMessage(vscode.l10n.t("Gregal: the number must be an integer."));
        return;
    }
    const repo = await vscode.window.showInputBox({ prompt: vscode.l10n.t("owner/repo (leave empty for the current repository)"), placeHolder: "solanes/gregal" });
    const query = selected.action === "list" ? await vscode.window.showInputBox({ prompt: vscode.l10n.t("Optional search"), placeHolder: "is:open" }) : "";
    const { base, token, session } = await connectionConfig();
    const out = vscode.window.createOutputChannel("Gregal GitHub");
    out.clear(); out.appendLine(`≋ ${kind.label} · ${selected.label}`); out.show(true);
    try {
        const data = await apiPost(base, token, "/api/github", {
            tool: kind.tool, action: selected.action, number: number ? Number(number) : 0,
            repo: repo || "", query: query || "",
        }, { session }) as { output?: string };
        out.appendLine(data.output || vscode.l10n.t("(no results)"));
    } catch (e) {
        void vscode.window.showErrorMessage(vscode.l10n.t("Gregal GitHub: {0}", e instanceof Error ? e.message : String(e)));
    }
}

export function activate(ctx: vscode.ExtensionContext): void {
    activateChat(ctx);
    ctx.subscriptions.push(
        vscode.commands.registerCommand("gregal.rewind", () => withIdleConversation(rewindCmd)),
        vscode.commands.registerCommand("gregal.github", githubCmd),
        vscode.commands.registerCommand("gregal.acceptSelectedHunk", () => selectedRangeCommand("git.stageSelectedRanges")),
        vscode.commands.registerCommand("gregal.rejectSelectedHunk", async () => {
            const answer = await vscode.window.showWarningMessage(
                vscode.l10n.t("Discard the selected lines from this Git diff?"), { modal: true }, vscode.l10n.t("Discard lines"));
            if (answer === vscode.l10n.t("Discard lines")) await selectedRangeCommand("git.revertSelectedRanges");
        }),
    );
}

async function selectedRangeCommand(command: string): Promise<void> {
    const available = await vscode.commands.getCommands(true);
    if (!available.includes(command)) {
        void vscode.window.showErrorMessage(vscode.l10n.t("The Git selected-range action is unavailable. Open a Git diff and select changed lines."));
        return;
    }
    try { await vscode.commands.executeCommand(command); }
    catch (error) {
        void vscode.window.showErrorMessage(vscode.l10n.t("Open a Git diff, select the changed lines, and try again. {0}", error instanceof Error ? error.message : String(error)));
    }
}

export function deactivate(): Promise<void> { return deactivateChat(); }
