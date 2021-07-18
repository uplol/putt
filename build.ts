import { Workspace, pushStep, sh } from "runtime/core.ts";
import * as Docker from "pkg/buildy/docker@1.0/mod.ts";

export async function build(ws: Workspace) {
  pushStep("Build Putt Binary");
  await Docker.run("go build -o putt cmd/putt/main.go", {
    image: `golang:1.16`,
    copy: ["cmd/**", "go.sum", "go.mod"],
  });
}
