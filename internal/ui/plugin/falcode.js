/**
 * falcode.js — OpenCode plugin for falcode status reporting.
 *
 * This plugin is auto-installed to ~/.config/opencode/plugins/falcode.js by
 * falcode on startup. It is a no-op when the FALCODE_STATUS_PIPE environment
 * variable is not set, so it is safe to install globally.
 *
 * When FALCODE_STATUS_PIPE is set, the plugin opens the named FIFO (pipe) for
 * writing and emits newline-delimited JSON events whenever OpenCode's session
 * status changes. falcode reads these events to update the workspace tab icon.
 *
 * Event schema (all fields are strings):
 *   { "type": "status", "status": "busy" }   — agent is actively processing
 *   { "type": "status", "status": "idle" }   — agent finished / idle
 *   { "type": "permission" }                  — agent awaiting permission grant
 *   { "type": "question" }                    — agent asking user a question
 *
 * OpenCode plugin API (from @opencode-ai/plugin):
 *   Export a default async function (PluginInput) => Hooks.
 *   The `event` hook on Hooks receives every server-sent event as { event }.
 *   The `tool.execute.before` hook fires before each tool call with { tool }.
 *   The `permission.ask` hook fires when the agent requests a permission.
 */

import { openSync, writeSync, constants } from "node:fs";

/** @param {import("@opencode-ai/plugin").PluginInput} _input */
export default async (_input) => {
  const pipePath = Bun.env.FALCODE_STATUS_PIPE;
  if (!pipePath) {
    return {};
  }

  let fd = -1;
  try {
    fd = openSync(pipePath, constants.O_WRONLY | constants.O_NONBLOCK);
  } catch {
    return {};
  }

  function writeEvent(evt) {
    if (fd < 0) return;
    try {
      writeSync(fd, JSON.stringify(evt) + "\n");
    } catch {
      fd = -1;
    }
  }

  return {
    async "permission.ask"(_input, _output) {
      writeEvent({ type: "permission" });
    },

    async "tool.execute.before"(input) {
      if (input.tool === "question") {
        writeEvent({ type: "question" });
      }
    },

    async event({ event }) {
      switch (event.type) {
        case "session.status": {
          const statusType = event.properties?.status?.type ?? "idle";
          writeEvent({ type: "status", status: statusType });
          break;
        }
        case "session.idle":
          writeEvent({ type: "status", status: "idle" });
          break;
        case "permission.replied":
          writeEvent({ type: "status", status: "busy" });
          break;
      }
    },
  };
};
