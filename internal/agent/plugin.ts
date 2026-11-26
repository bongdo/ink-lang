import { Plugin } from "@opencode/plugin"
import { execFileSync } from "node:child_process"

const INK = __INK__
const last = new Map<string, string>()

export default Plugin.define({
  id: "ink",
  async setup(ctx) {
    await ctx.tool.hook("execute.after", async (event: any) => {
      if (event.status !== "completed") return
      if (!["edit", "write", "patch"].includes(event.tool)) return
      const file = event.input?.path ?? event.input?.filePath
      if (!file || !String(file).endsWith(".ink")) return
      let out = ""
      try {
        out = execFileSync(INK[0], [...INK.slice(1), "check", "--json"], { cwd: ctx.location.directory, encoding: "utf8" })
      } catch (e: any) {
        out = e.stdout ?? ""
      }
      let diags: any[] = []
      try {
        diags = JSON.parse(out || "[]")
      } catch {
        return
      }
      const mine = diags.filter((d) => d.file === file)
      const text = mine
        .map((d) => `${d.file}:${d.line}:${d.col} ${d.code}: ${d.message}${d.hint ? "\n  hint: " + d.hint : ""}`)
        .join("\n")
      if (!text || last.get(event.sessionID) === text) return
      last.set(event.sessionID, text)
      await ctx.session.synthetic({
        sessionID: event.sessionID,
        text: "Ink compiler diagnostics after the last edit:\n" + text,
      })
    })
  },
})
