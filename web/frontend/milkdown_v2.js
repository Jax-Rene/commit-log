import { Crepe } from "@milkdown/crepe";
import {
  commandsCtx,
  editorViewCtx,
  editorViewOptionsCtx,
} from "@milkdown/kit/core";
import { TextSelection } from "@milkdown/kit/prose/state";
import { listenerCtx } from "@milkdown/kit/plugin/listener";
import { cursor } from "@milkdown/plugin-cursor";
import { math } from "@milkdown/plugin-math";
import { upload, uploadConfig } from "@milkdown/plugin-upload";
import { $nodeSchema, $remark, $view, replaceAll } from "@milkdown/kit/utils";
import commonStyleUrl from "@milkdown/crepe/theme/common/style.css?url";
import nordStyleUrl from "@milkdown/crepe/theme/nord.css?url";
import katexStyleUrl from "katex/dist/katex.min.css?url";
import { toggleLinkCommand } from "@milkdown/kit/component/link-tooltip";

const styleUrls = [
  commonStyleUrl,
  nordStyleUrl,
  typeof katexStyleUrl === "undefined" ? null : katexStyleUrl,
].filter(Boolean);

const INLINE_AI_CHAT_ICON = `
  <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24">
    <path fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" d="M12 3l1.9 4.6L18.8 9l-3.4 3 1 4.9L12 14.8 7.6 17.9l1-4.9L5.2 9l4.9-.4L12 3z" />
  </svg>
`;

const VIDEO_EMBED_ALLOW =
  "accelerometer; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share";
const BILIBILI_IFRAME_SANDBOX =
  "allow-scripts allow-same-origin allow-presentation";
const VIDEO_EMBED_LOADING_CLASS = "is-loading";
const VIDEO_EMBED_READY_CLASS = "is-ready";
const VIDEO_EMBED_PLATFORMS = {
  youtube: { label: "YouTube", aspect: "16:9" },
  bilibili: { label: "B 站", aspect: "16:9" },
  douyin: { label: "抖音", aspect: "9:16" },
};

let inlineAIToolbarHandler = null;

function markVideoEmbedReady(container) {
  if (!container || !container.classList) {
    return;
  }
  container.classList.remove(VIDEO_EMBED_LOADING_CLASS);
  container.classList.add(VIDEO_EMBED_READY_CLASS);
}

function bindVideoEmbedLoading(container) {
  if (
    !container ||
    !container.classList ||
    typeof container.querySelector !== "function"
  ) {
    return;
  }
  const iframe = container.querySelector("iframe");
  if (!iframe) {
    markVideoEmbedReady(container);
    return;
  }

  if (!container.dataset) {
    container.dataset = {};
  }
  if (container.dataset.videoEmbedLoadingBound === "true") {
    return;
  }
  container.dataset.videoEmbedLoadingBound = "true";
  container.classList.remove(VIDEO_EMBED_READY_CLASS);
  container.classList.add(VIDEO_EMBED_LOADING_CLASS);

  const onReady = () => {
    markVideoEmbedReady(container);
  };

  iframe.addEventListener("load", onReady, { once: true });
  iframe.addEventListener("error", onReady, { once: true });
}

function initializeVideoEmbedLoadingState(root = document) {
  if (!root || typeof root.querySelectorAll !== "function") {
    return 0;
  }
  const containers = root.querySelectorAll("[data-video-embed]");
  if (!containers || typeof containers.forEach !== "function") {
    return 0;
  }
  containers.forEach((container) => {
    bindVideoEmbedLoading(container);
  });
  return containers.length;
}

function ensureStyles() {
  styleUrls.forEach((href, index) => {
    if (!href) {
      return;
    }
    const id = `milkdown-crepe-style-${index}`;
    if (document.getElementById(id)) {
      return;
    }
    const link = document.createElement("link");
    link.id = id;
    link.rel = "stylesheet";
    link.href = href;
    document.head.appendChild(link);
  });
}

function normalizePreviewFeatures(features) {
  const featureFlags =
    typeof features === "object" && features !== null ? { ...features } : {};
  const Feature = Crepe.Feature || {};
  const disabledDefaults = {};
  if (Feature.BlockEdit) {
    disabledDefaults[Feature.BlockEdit] = false;
  }
  if (Feature.Toolbar) {
    disabledDefaults[Feature.Toolbar] = false;
  }
  if (Feature.Placeholder) {
    disabledDefaults[Feature.Placeholder] = false;
  }
  return { ...disabledDefaults, ...featureFlags };
}

function applyMarkdownToInstance(
  instance,
  markdown,
  { flush = false, silent = false } = {},
) {
  if (!instance || typeof instance.editor?.action !== "function") {
    return false;
  }
  const normalized = typeof markdown === "string" ? markdown : "";
  try {
    instance.editor.action((ctx) => {
      const command = replaceAll(normalized, flush);
      if (typeof command === "function") {
        command(ctx);
      }
      const view =
        ctx && typeof ctx.get === "function" ? ctx.get(editorViewCtx) : null;
      if (view) {
        convertBlockquoteCallouts(view);
        convertParagraphVideoEmbeds(view);
        initializeVideoEmbedLoadingState(view.dom);
      }
    });
    return true;
  } catch (error) {
    if (!silent) {
      console.warn("[milkdown] Markdown 更新失败", error);
    }
    return false;
  }
}

function ensureSetMarkdown(instance, options = {}) {
  const { flush = false, silent = false } = options;
  if (!instance || typeof instance.editor?.action !== "function") {
    return instance;
  }
  const existing = instance.setMarkdown;
  if (typeof existing === "function" && existing.__commitLogPatched__) {
    return instance;
  }
  const setter = async (markdown, override = {}) => {
    const shouldFlush =
      typeof override.flush === "boolean" ? override.flush : flush;
    const shouldSilence =
      typeof override.silent === "boolean" ? override.silent : silent;
    return applyMarkdownToInstance(instance, markdown, {
      flush: shouldFlush,
      silent: shouldSilence,
    });
  };
  setter.__commitLogPatched__ = true;
  Object.defineProperty(instance, "setMarkdown", {
    configurable: true,
    enumerable: false,
    writable: true,
    value: setter,
  });
  return instance;
}

async function createReadOnlyViewer({
  mount,
  markdown = "",
  features,
  featureConfigs,
} = {}) {
  if (!mount) {
    throw new Error("[milkdown] 预览需要有效的挂载节点");
  }
  ensureStyles();
  const normalized = typeof markdown === "string" ? markdown : "";
  const preview = new Crepe({
    root: mount,
    defaultValue: normalized.trim() ? normalized : "# ",
    features: normalizePreviewFeatures(features),
    featureConfigs,
  });
  usePlugins(preview.editor, [
    math,
    videoEmbedSchema,
    videoEmbedRemark,
    calloutSchema,
    calloutRemark,
    calloutView,
  ]);
  ensureSetMarkdown(preview, { flush: true, silent: true });
  if (typeof preview.setReadonly === "function") {
    preview.setReadonly(true);
  }
  mount.innerHTML = "";
  await preview.create();
  if (typeof preview.setMarkdown === "function") {
    try {
      const result = preview.setMarkdown(normalized);
      if (result && typeof result.then === "function") {
        await result;
      }
    } catch (error) {
      console.warn("[milkdown] 预览内容设置失败", error);
    }
  }
  initializeVideoEmbedLoadingState(mount);
  return preview;
}

if (typeof window !== "undefined") {
  window.MilkdownV2 = window.MilkdownV2 || {};
  if (typeof window.MilkdownV2.createReadOnlyViewer !== "function") {
    window.MilkdownV2.createReadOnlyViewer = createReadOnlyViewer;
  }
  window.MilkdownV2.deriveTitleFromMarkdown = deriveTitleFromMarkdown;
}

function getInitialMarkdown() {
  if (typeof window !== "undefined" && window.__MILKDOWN_V2__) {
    const { content } = window.__MILKDOWN_V2__;
    if (typeof content === "string") {
      return content.trim().length > 0 ? content : "# ";
    }
  }
  return "# ";
}

const DEFAULT_CHANGE_POLL_INTERVAL = 3000;
const DEFAULT_AUTOSAVE_INTERVAL = 60000;

function generateDraftSessionId() {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return `session-${Date.now().toString(36)}-${Math.random()
    .toString(36)
    .slice(2, 10)}`;
}

function createToast() {
  if (window.AdminUI && typeof window.AdminUI.toast === "function") {
    return window.AdminUI.toast;
  }
  return ({ type, message }) => {
    const prefix = type ? `[${type}]` : "[info]";
    console.log(prefix, message);
  };
}

function coerceNumber(value, fallback = 0) {
  const parsed = typeof value === "string" ? Number(value) : value;
  return Number.isFinite(parsed) ? parsed : fallback;
}

function coerceString(value, fallback = "") {
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "number") {
    return String(value);
  }
  return fallback;
}

function deriveTitleFromMarkdown(content) {
  const source = coerceString(content, "");
  if (!source) {
    return "";
  }
  const newlineIndex = source.indexOf("\n");
  const firstLine = newlineIndex >= 0 ? source.slice(0, newlineIndex) : source;
  let trimmed = firstLine.trim();
  if (!trimmed) {
    return "";
  }
  if (trimmed.startsWith("#")) {
    trimmed = trimmed.replace(/^#+/, "").trim();
    trimmed = trimmed.replace(/#+$/, "").trim();
  }
  return trimmed;
}

function parseVideoEmbedSource(value) {
  let raw = typeof value === "string" ? value.trim() : "";
  if (!raw) {
    return null;
  }
  if (raw.startsWith("<") && raw.endsWith(">")) {
    raw = raw.slice(1, -1).trim();
  }
  if (!raw) {
    return null;
  }
  raw = normalizeVideoURL(raw);
  const parsed = safeParseUrl(raw);
  if (!parsed) {
    return null;
  }
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    return null;
  }
  return (
    parseYouTubeEmbed(parsed, raw) ||
    parseBilibiliEmbed(parsed, raw) ||
    parseDouyinEmbed(parsed, raw)
  );
}

function safeParseUrl(value) {
  try {
    return new URL(value);
  } catch (error) {
    return null;
  }
}

function normalizeVideoURL(value) {
  if (typeof value !== "string" || !value) {
    return value;
  }
  const lower = value.toLowerCase();
  if (lower.startsWith("http://") || lower.startsWith("https://")) {
    return value;
  }
  const knownPrefixes = [
    "douyin.com/",
    "www.douyin.com/",
    "iesdouyin.com/",
    "www.iesdouyin.com/",
    "v.douyin.com/",
    "bilibili.com/",
    "www.bilibili.com/",
    "youtube.com/",
    "www.youtube.com/",
    "youtu.be/",
  ];
  if (knownPrefixes.some((prefix) => lower.startsWith(prefix))) {
    return `https://${value}`;
  }
  return value;
}

function stripLeadingSlash(value) {
  if (typeof value !== "string") {
    return "";
  }
  let normalized = value;
  while (normalized.startsWith("/")) {
    normalized = normalized.slice(1);
  }
  return normalized;
}

function isHostOrSubdomain(hostname, domain) {
  const host =
    typeof hostname === "string" ? hostname.trim().toLowerCase() : "";
  const expected =
    typeof domain === "string" ? domain.trim().toLowerCase() : "";
  if (!host || !expected) {
    return false;
  }
  return host === expected || host.endsWith(`.${expected}`);
}

function parseYouTubeEmbed(url, source) {
  const host = url.hostname.toLowerCase();
  let videoId = "";
  if (host === "youtu.be") {
    videoId = stripLeadingSlash(url.pathname).split("/")[0] || "";
  } else if (isHostOrSubdomain(host, "youtube.com")) {
    const path = stripLeadingSlash(url.pathname);
    if (path === "watch") {
      videoId = url.searchParams.get("v") || "";
    } else if (path.startsWith("shorts/")) {
      videoId = path.replace("shorts/", "").split("/")[0] || "";
    } else if (path.startsWith("embed/")) {
      videoId = path.replace("embed/", "").split("/")[0] || "";
    } else if (path.startsWith("live/")) {
      videoId = path.replace("live/", "").split("/")[0] || "";
    }
  }
  if (!videoId) {
    return null;
  }
  const embed = new URL(`https://www.youtube.com/embed/${videoId}`);
  embed.searchParams.set("rel", "0");
  embed.searchParams.set("modestbranding", "1");
  embed.searchParams.set("playsinline", "1");
  const start = parseYouTubeStartSeconds(
    url.searchParams.get("start") || url.searchParams.get("t") || "",
  );
  if (start > 0) {
    embed.searchParams.set("start", String(start));
  }
  return {
    platform: "youtube",
    source,
    embed: embed.toString(),
    aspect: VIDEO_EMBED_PLATFORMS.youtube.aspect,
  };
}

function parseYouTubeStartSeconds(value) {
  const raw = typeof value === "string" ? value.trim() : "";
  if (!raw) {
    return 0;
  }
  if (/^\\d+$/.test(raw)) {
    const seconds = Number(raw);
    return Number.isFinite(seconds) && seconds > 0 ? seconds : 0;
  }
  let total = 0;
  const matches = raw.match(/(\\d+)(h|m|s)/gi);
  if (!matches) {
    return 0;
  }
  matches.forEach((chunk) => {
    const match = /(\\d+)(h|m|s)/i.exec(chunk);
    if (!match) {
      return;
    }
    const amount = Number(match[1]);
    if (!Number.isFinite(amount) || amount <= 0) {
      return;
    }
    switch (match[2].toLowerCase()) {
      case "h":
        total += amount * 3600;
        break;
      case "m":
        total += amount * 60;
        break;
      case "s":
        total += amount;
        break;
      default:
        break;
    }
  });
  return total;
}

function parseBilibiliEmbed(url, source) {
  const host = url.hostname.toLowerCase();
  if (!isHostOrSubdomain(host, "bilibili.com")) {
    return null;
  }
  const segments = stripLeadingSlash(url.pathname).split("/");
  if (segments[0] !== "video" || !segments[1]) {
    return null;
  }
  const rawId = segments[1];
  const lower = rawId.toLowerCase();
  const params = new URLSearchParams();
  if (lower.startsWith("bv")) {
    params.set("bvid", rawId);
  } else if (lower.startsWith("av")) {
    params.set("aid", lower.replace("av", ""));
  } else if (/^\\d+$/.test(rawId)) {
    params.set("aid", rawId);
  } else {
    return null;
  }
  const page = Number(url.searchParams.get("p"));
  params.set("page", Number.isFinite(page) && page > 0 ? String(page) : "1");
  params.set("high_quality", "1");
  params.set("danmaku", "0");
  params.set("autoplay", "0");
  const embed = `https://player.bilibili.com/player.html?${params.toString()}`;
  return {
    platform: "bilibili",
    source,
    embed,
    aspect: VIDEO_EMBED_PLATFORMS.bilibili.aspect,
  };
}

function parseDouyinEmbed(url, source) {
  const host = url.hostname.toLowerCase();
  if (
    !isHostOrSubdomain(host, "douyin.com") &&
    !isHostOrSubdomain(host, "iesdouyin.com")
  ) {
    return null;
  }
  const segments = stripLeadingSlash(url.pathname).split("/");
  let videoId = "";
  segments.forEach((segment, index) => {
    if (segment === "video" && index + 1 < segments.length) {
      videoId = segments[index + 1];
    }
    if (segment.startsWith("modal_id=")) {
      videoId = segment.replace("modal_id=", "");
    }
  });
  if (!videoId) {
    videoId = url.searchParams.get("modal_id") || "";
  }
  let embed = "";
  if (videoId) {
    embed = `https://www.iesdouyin.com/share/video/${videoId}`;
  } else if (host === "v.douyin.com") {
    embed = source;
  } else {
    return null;
  }
  return {
    platform: "douyin",
    source,
    embed,
    aspect: VIDEO_EMBED_PLATFORMS.douyin.aspect,
  };
}

function buildVideoEmbedTitle(platform) {
  const entry = VIDEO_EMBED_PLATFORMS[platform];
  if (!entry) {
    return "视频播放器";
  }
  return `${entry.label} 视频播放器`;
}

function videoEmbedSandboxForPlatform(platform) {
  return platform === "bilibili" ? BILIBILI_IFRAME_SANDBOX : "";
}

function synchronizeTitleHeading(markdown, title) {
  const source = coerceString(markdown, "");
  const normalizedTitle = coerceString(title, "").trim();
  const lines = source.split("\n");

  let index = 0;
  while (index < lines.length && lines[index].trim() === "") {
    index++;
  }

  if (index < lines.length && lines[index].trim() === "---") {
    index++;
    while (index < lines.length && lines[index].trim() !== "---") {
      index++;
    }
    if (index < lines.length) {
      index++;
    }
  }

  const prefix = lines.slice(0, index);
  let rest = lines.slice(index);

  while (rest.length > 0 && rest[0].trim() === "") {
    rest.shift();
  }

  if (!normalizedTitle) {
    if (rest.length > 0 && rest[0].trim().startsWith("#")) {
      rest.shift();
      while (rest.length > 0 && rest[0].trim() === "") {
        rest.shift();
      }
      const result = [...prefix];
      if (rest.length > 0) {
        if (result.length > 0 && result[result.length - 1].trim() !== "") {
          result.push("");
        }
        result.push(...rest);
      }
      return result.join("\n");
    }
    return source;
  }

  if (rest.length > 0 && rest[0].trim().startsWith("#")) {
    rest.shift();
  }
  while (rest.length > 0 && rest[0].trim() === "") {
    rest.shift();
  }

  const result = [...prefix];
  if (result.length > 0 && result[result.length - 1].trim() !== "") {
    result.push("");
  }
  result.push(`# ${normalizedTitle}`);
  if (rest.length > 0) {
    result.push("");
    result.push(...rest);
  }
  return result.join("\n");
}

function pickProperty(source, candidates, fallback) {
  if (!source || typeof source !== "object") {
    return fallback;
  }
  for (const key of candidates) {
    if (Object.prototype.hasOwnProperty.call(source, key)) {
      const value = source[key];
      if (value !== undefined && value !== null) {
        return value;
      }
    }
  }
  return fallback;
}

function cloneValue(value, options = {}) {
  const { skipStructuredClone = false } = options;
  if (!skipStructuredClone && typeof structuredClone === "function") {
    try {
      return structuredClone(value);
    } catch (error) {
      console.warn("[milkdown] 复制数据失败，将使用回退方案", error);
    }
  }
  try {
    return JSON.parse(JSON.stringify(value));
  } catch (error) {
    console.warn("[milkdown] JSON 克隆数据失败，将返回原始引用", error);
  }
  return value;
}

function calculateContentMetrics(markdown) {
  const source = coerceString(markdown, "");
  if (!source.trim()) {
    return { words: 0, characters: 0, paragraphs: 0 };
  }

  const withoutCode = source
    .replace(/```[\s\S]*?```/g, " ")
    .replace(/`[^`]*`/g, " ");

  const withoutLinks = withoutCode
    .replace(/!\[[^\]]*\]\([^)]*\)/g, " ")
    .replace(/\[[^\]]*\]\([^)]*\)/g, "$1 ");

  const normalized = withoutLinks
    .replace(/[#>*_~`\-]+/g, " ")
    .replace(/\d+\./g, " ")
    .replace(/&[a-z]+;/gi, " ")
    .replace(/\r/g, "\n");

  const paragraphCount = normalized
    .split(/\n{2,}/)
    .map((block) => block.trim())
    .filter(Boolean).length;

  const collapsed = normalized
    .replace(/\n+/g, " ")
    .replace(/[\u200B-\u200D\uFEFF]/g, "")
    .replace(
      /[^\p{Letter}\p{Number}\p{Mark}\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Hangul}]+/gu,
      " ",
    )
    .trim();

  if (!collapsed) {
    return { words: 0, characters: 0, paragraphs: paragraphCount };
  }

  const segments = collapsed.split(/\s+/).filter(Boolean);
  let words = 0;
  let characters = 0;

  for (const segment of segments) {
    const cjkMatches = segment.match(
      /[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Hangul}]/gu,
    );
    if (cjkMatches && cjkMatches.length > 0) {
      words += cjkMatches.length;
      characters += cjkMatches.length;
    }

    const remaining = segment.replace(
      /[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Hangul}]/gu,
      "",
    );
    const normalizedRemaining = remaining.replace(/\s+/g, "");
    if (normalizedRemaining.length > 0) {
      words += 1;
      characters += normalizedRemaining.length;
    } else if (!cjkMatches) {
      words += 1;
      characters += segment.length;
    }
  }

  return {
    words,
    characters,
    paragraphs: paragraphCount,
  };
}

function clamp(value, min, max) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric)) {
    return min;
  }
  if (numeric < min) {
    return min;
  }
  if (numeric > max) {
    return max;
  }
  return numeric;
}

function normalizeSelectionContent(value) {
  if (typeof value !== "string") {
    return "";
  }
  return value.replace(/\r\n/g, "\n").replace(/\u00a0/g, " ");
}

function splitMarkdownTableRow(line) {
  if (typeof line !== "string") {
    return [];
  }
  let content = line.trim();
  if (!content) {
    return [];
  }
  while (content.startsWith("|")) {
    content = content.slice(1);
    content = content.trimStart();
  }
  while (content.endsWith("|")) {
    content = content.slice(0, -1);
    content = content.trimEnd();
  }
  if (!content) {
    return [""];
  }
  const cells = [];
  let current = "";
  for (let index = 0; index < content.length; index += 1) {
    const char = content[index];
    if (char === "\\" && index + 1 < content.length) {
      const nextChar = content[index + 1];
      if (nextChar === "|") {
        current += "|";
        index += 1;
        continue;
      }
      current += char;
      continue;
    }
    if (char === "|") {
      cells.push(current.trim());
      current = "";
      continue;
    }
    current += char;
  }
  cells.push(current.trim());
  return cells;
}

function isMarkdownAlignmentToken(token) {
  if (typeof token !== "string") {
    return false;
  }
  const normalized = token.replace(/\s+/g, "");
  if (!normalized) {
    return false;
  }
  return /^:?-{3,}:?$/.test(normalized);
}

function alignmentFromToken(token) {
  const normalized = typeof token === "string" ? token.trim() : "";
  if (!normalized) {
    return "left";
  }
  const hasLeft = normalized.startsWith(":");
  const hasRight = normalized.endsWith(":");
  if (hasLeft && hasRight) {
    return "center";
  }
  if (hasRight) {
    return "right";
  }
  if (hasLeft) {
    return "left";
  }
  return "left";
}

function parseMarkdownTable(text) {
  if (typeof text !== "string") {
    return null;
  }
  const rawLines = text.replace(/\r/g, "\n").split("\n");
  const lines = rawLines.map((line) => line.trim()).filter(Boolean);
  if (lines.length < 3) {
    return null;
  }
  const headerCells = splitMarkdownTableRow(lines[0]);
  const alignmentCells = splitMarkdownTableRow(lines[1]);
  if (headerCells.length < 2 || alignmentCells.length !== headerCells.length) {
    return null;
  }
  if (!alignmentCells.every(isMarkdownAlignmentToken)) {
    return null;
  }
  const rows = lines
    .slice(2)
    .filter(Boolean)
    .map((line) => {
      const cells = splitMarkdownTableRow(line);
      const normalized = [];
      for (let index = 0; index < headerCells.length; index += 1) {
        normalized.push((cells[index] ?? "").trim());
      }
      return normalized;
    });
  if (!rows.length) {
    return null;
  }
  return {
    headers: headerCells.map((cell) => cell.trim()),
    alignments: alignmentCells.map(alignmentFromToken),
    rows,
  };
}

function buildTableNodeFromMarkdown(schema, tableData) {
  if (!schema || !tableData) {
    return null;
  }
  const tableType = pickProperty(schema.nodes, ["table"], null);
  const headerRowType = pickProperty(
    schema.nodes,
    ["table_header_row", "tableHeaderRow"],
    null,
  );
  const bodyRowType = pickProperty(
    schema.nodes,
    ["table_row", "tableRow"],
    null,
  );
  const headerCellType = pickProperty(
    schema.nodes,
    ["table_header", "tableHeader"],
    null,
  );
  const bodyCellType = pickProperty(
    schema.nodes,
    ["table_cell", "tableCell"],
    null,
  );
  const paragraphType = pickProperty(schema.nodes, ["paragraph"], null);
  if (
    !tableType ||
    !headerRowType ||
    !bodyRowType ||
    !headerCellType ||
    !bodyCellType ||
    !paragraphType
  ) {
    return null;
  }
  const columnCount = tableData.headers.length;
  const alignments = tableData.alignments;
  const createParagraph = (value) => {
    const trimmed = typeof value === "string" ? value.trim() : "";
    if (trimmed) {
      return paragraphType.create(null, schema.text(trimmed));
    }
    if (typeof paragraphType.createAndFill === "function") {
      const fallback = paragraphType.createAndFill();
      if (fallback) {
        return fallback;
      }
    }
    return paragraphType.create(null);
  };
  const createCell = (cellType, value, columnIndex) => {
    const paragraph = createParagraph(value);
    const alignment = alignments[columnIndex] || "left";
    return cellType.create({ alignment }, paragraph ? [paragraph] : undefined);
  };
  const headerCells = tableData.headers.map((value, index) =>
    createCell(headerCellType, value, index),
  );
  const headerRow = headerRowType.create(null, headerCells);
  const bodyRows = tableData.rows.map((rowValues) => {
    const normalized = [];
    for (let columnIndex = 0; columnIndex < columnCount; columnIndex += 1) {
      normalized.push(rowValues[columnIndex] ?? "");
    }
    const cells = normalized.map((value, index) =>
      createCell(bodyCellType, value, index),
    );
    return bodyRowType.create(null, cells);
  });
  return tableType.create(null, [headerRow, ...bodyRows]);
}

// 处理 Markdown 表格黏贴，自动插入结构化表格
function handleMarkdownTablePaste(view, event) {
  if (!view || !event || !event.clipboardData) {
    return false;
  }
  const clipboardTypes = Array.from(event.clipboardData.types || []);
  if (clipboardTypes.includes("application/x-prosemirror-slice")) {
    return false;
  }
  if (clipboardTypes.includes("text/html")) {
    const html = event.clipboardData.getData("text/html") || "";
    if (/<table[\s>]/i.test(html)) {
      return false;
    }
  }
  const text = event.clipboardData.getData("text/plain");
  if (!text || text.indexOf("|") === -1) {
    return false;
  }
  const tableData = parseMarkdownTable(text);
  if (!tableData) {
    return false;
  }
  const tableNode = buildTableNodeFromMarkdown(view.state?.schema, tableData);
  if (!tableNode) {
    return false;
  }
  event.preventDefault();
  const transaction = view.state.tr
    .replaceSelectionWith(tableNode)
    .scrollIntoView();
  view.dispatch(transaction);
  view.focus();
  return true;
}

function handleVideoEmbedPaste(view, event) {
  if (!view || !event || !event.clipboardData) {
    return false;
  }
  const clipboardTypes = Array.from(event.clipboardData.types || []);
  if (clipboardTypes.includes("application/x-prosemirror-slice")) {
    return false;
  }
  const text = event.clipboardData.getData("text/plain");
  if (!text) {
    return false;
  }
  const trimmed = text.trim();
  if (!trimmed || trimmed.includes("\n")) {
    return false;
  }
  if (trimmed.split(/\s+/).length !== 1) {
    return false;
  }
  const embed = parseVideoEmbedSource(trimmed);
  if (!embed) {
    return false;
  }
  const videoType = view.state?.schema?.nodes?.video_embed;
  if (!videoType || typeof videoType.create !== "function") {
    return false;
  }
  event.preventDefault();
  const node = videoType.create({
    platform: embed.platform,
    source: embed.source,
    embed: embed.embed,
    aspect: embed.aspect,
  });
  const transaction = view.state.tr
    .replaceSelectionWith(node)
    .scrollIntoView();
  view.dispatch(transaction);
  initializeVideoEmbedLoadingState(view.dom);
  view.focus();
  return true;
}

// 预置 Emoji 选项，配合斜杠菜单实现快捷搜索
const EMOJI_SLASH_ITEMS = [
  { key: "grinning-face", emoji: "😀", label: "微笑 smile happy" },
  {
    key: "grinning-face-with-smiling-eyes",
    emoji: "😁",
    label: "露齿笑 grin",
  },
  { key: "face-with-tears-of-joy", emoji: "😂", label: "喜极而泣 joy lol" },
  {
    key: "rolling-on-the-floor-laughing",
    emoji: "🤣",
    label: "笑到打滚 rofl",
  },
  {
    key: "smiling-face-with-smiling-eyes",
    emoji: "😊",
    label: "害羞微笑 blush",
  },
  { key: "winking-face", emoji: "😉", label: "眨眼 wink" },
  {
    key: "smiling-face-with-heart-eyes",
    emoji: "😍",
    label: "花痴 love heart-eyes",
  },
  {
    key: "smiling-face-with-sunglasses",
    emoji: "😎",
    label: "酷 cool sunglasses",
  },
  { key: "thinking-face", emoji: "🤔", label: "思考 thinking question" },
  { key: "neutral-face", emoji: "😐", label: "无语 neutral" },
  { key: "expressionless-face", emoji: "😑", label: "面瘫 expressionless" },
  { key: "sleeping-face", emoji: "😴", label: "睡觉 sleepy sleep" },
  { key: "crying-face", emoji: "😢", label: "哭 sad cry" },
  { key: "loudly-crying-face", emoji: "😭", label: "嚎啕大哭 sob" },
  { key: "pouting-face", emoji: "😡", label: "生气 angry" },
  { key: "face-with-symbols-on-mouth", emoji: "🤬", label: "暴怒 rage" },
  { key: "face-with-open-mouth", emoji: "😮", label: "惊讶 surprised" },
  { key: "astonished-face", emoji: "😲", label: "震惊 astonished" },
  { key: "partying-face", emoji: "🥳", label: "派对 celebrate party" },
  { key: "hugging-face", emoji: "🤗", label: "拥抱 hug" },
  { key: "folded-hands", emoji: "🙏", label: "感谢 thank pray" },
  { key: "thumbs-up", emoji: "👍", label: "点赞 good thumbs-up" },
  { key: "thumbs-down", emoji: "👎", label: "点踩 thumbs-down" },
  { key: "clapping-hands", emoji: "👏", label: "鼓掌 clap bravo" },
  { key: "ok-hand", emoji: "👌", label: "OK perfect" },
  { key: "flexed-biceps", emoji: "💪", label: "加油 muscle strong" },
  { key: "fire", emoji: "🔥", label: "火 hot fire" },
  { key: "glowing-star", emoji: "✨", label: "闪耀 sparkles" },
  { key: "white-medium-star", emoji: "⭐", label: "星星 star" },
  { key: "light-bulb", emoji: "💡", label: "灵感 idea bulb" },
  { key: "warning", emoji: "⚠️", label: "警告 warning" },
  { key: "check-mark-button", emoji: "✅", label: "完成 done check" },
  { key: "cross-mark", emoji: "❌", label: "否决 cross" },
  { key: "question-mark", emoji: "❓", label: "疑问 question help" },
  { key: "high-voltage", emoji: "⚡", label: "电力 energy lightning" },
  { key: "rocket", emoji: "🚀", label: "火箭 rocket launch" },
  { key: "party-popper", emoji: "🎉", label: "庆祝 celebrate tada" },
  { key: "wrapped-gift", emoji: "🎁", label: "礼物 gift" },
  { key: "calendar", emoji: "📅", label: "日程 calendar schedule" },
  { key: "memo", emoji: "📝", label: "记录 memo note" },
];

const CALLOUT_MARKER = "[!callout]";
const CALLOUT_DEFAULT_EMOJI = "💡";
const CALLOUT_EMOJI_SET = new Set(
  EMOJI_SLASH_ITEMS.map((item) => item?.emoji).filter(Boolean),
);
const CALLOUT_EMOJI_LIST = Array.from(CALLOUT_EMOJI_SET).sort(
  (a, b) => b.length - a.length,
);

function normalizeCalloutEmoji(emoji) {
  if (typeof emoji !== "string") {
    return "";
  }
  const trimmed = emoji.trim();
  return CALLOUT_EMOJI_SET.has(trimmed) ? trimmed : "";
}

function buildCalloutMarker(emoji) {
  const normalized = normalizeCalloutEmoji(emoji);
  return normalized ? `${CALLOUT_MARKER} ${normalized}` : CALLOUT_MARKER;
}

function parseCalloutMarker(text) {
  if (typeof text !== "string") {
    return null;
  }
  const match = text.match(/^\s*\[!callout\]\s*(.*)$/i);
  if (!match) {
    return null;
  }
  const raw = typeof match[1] === "string" ? match[1].trim() : "";
  if (!raw) {
    return { emoji: "", text: "" };
  }
  let emoji = "";
  let remaining = raw;
  for (const candidate of CALLOUT_EMOJI_LIST) {
    if (raw.startsWith(candidate)) {
      emoji = candidate;
      remaining = raw.slice(candidate.length).trim();
      break;
    }
  }
  return { emoji, text: remaining };
}

function extractParagraphText(node) {
  if (!node || node.type !== "paragraph") {
    return "";
  }
  const collect = (current) => {
    if (!current) {
      return "";
    }
    if (current.type === "text" && typeof current.value === "string") {
      return current.value;
    }
    if (Array.isArray(current.children)) {
      return current.children.map(collect).join("");
    }
    return "";
  };
  return collect(node);
}

function createCalloutParagraph(text) {
  const value = typeof text === "string" ? text : "";
  return {
    type: "paragraph",
    children: value ? [{ type: "text", value }] : [],
  };
}

function readParagraphText(node) {
  if (!node) {
    return "";
  }
  if (typeof node.textBetween === "function" && node.content) {
    return node.textBetween(0, node.content.size, "\n", "\n");
  }
  if (typeof node.textContent === "string") {
    return node.textContent;
  }
  return "";
}

function buildCalloutNodeFromBlockquote(blockquote, schema) {
  if (!blockquote || !schema) {
    return null;
  }
  const blockquoteType = schema.nodes?.blockquote;
  const paragraphType = schema.nodes?.paragraph;
  const calloutType = schema.nodes?.callout;
  if (
    !blockquoteType ||
    !paragraphType ||
    !calloutType ||
    blockquote.type !== blockquoteType
  ) {
    return null;
  }
  const firstChild = blockquote.firstChild;
  if (!firstChild || firstChild.type !== paragraphType) {
    return null;
  }
  const marker = parseCalloutMarker(readParagraphText(firstChild));
  if (!marker) {
    return null;
  }

  const children = [];
  const textValue = marker.text;
  if (textValue) {
    const textNode =
      typeof schema.text === "function" ? schema.text(textValue) : null;
    if (textNode) {
      children.push(paragraphType.create(null, textNode));
    } else {
      children.push(paragraphType.create(null));
    }
  }
  const childCount = blockquote.childCount || 0;
  for (let index = 1; index < childCount; index += 1) {
    children.push(blockquote.child(index));
  }
  if (!children.length) {
    children.push(paragraphType.create(null));
  }
  return calloutType.create({ emoji: marker.emoji }, children);
}

function convertBlockquoteCallouts(view) {
  if (!view || !view.state || !view.dispatch) {
    return false;
  }
  const { doc, schema } = view.state;
  const blockquoteType = schema?.nodes?.blockquote;
  const calloutType = schema?.nodes?.callout;
  if (!doc || !blockquoteType || !calloutType) {
    return false;
  }
  const replacements = [];
  doc.descendants((node, pos) => {
    if (node.type !== blockquoteType) {
      return;
    }
    const calloutNode = buildCalloutNodeFromBlockquote(node, schema);
    if (!calloutNode) {
      return;
    }
    replacements.push({
      from: pos,
      to: pos + node.nodeSize,
      node: calloutNode,
    });
  });
  if (!replacements.length) {
    return false;
  }
  let tr = view.state.tr;
  replacements
    .sort((a, b) => b.from - a.from)
    .forEach((replacement) => {
      tr = tr.replaceWith(replacement.from, replacement.to, replacement.node);
    });
  if (tr.docChanged) {
    view.dispatch(tr);
    return true;
  }
  return false;
}

function convertParagraphVideoEmbeds(view) {
  if (!view || !view.state || !view.dispatch) {
    return false;
  }
  const { doc, schema } = view.state;
  const paragraphType = schema?.nodes?.paragraph;
  const videoType = schema?.nodes?.video_embed;
  if (!doc || !paragraphType || !videoType) {
    return false;
  }
  const replacements = [];
  doc.descendants((node, pos, parent) => {
    if (parent && parent !== doc) {
      return;
    }
    if (!node || node.type !== paragraphType) {
      return;
    }
    const text = readParagraphText(node).trim();
    if (!text || text.includes("\n")) {
      return;
    }
    const embed = parseVideoEmbedSource(text);
    if (!embed) {
      return;
    }
    const videoNode = videoType.create({
      platform: embed.platform,
      source: embed.source,
      embed: embed.embed,
      aspect: embed.aspect,
    });
    replacements.push({
      from: pos,
      to: pos + node.nodeSize,
      node: videoNode,
    });
  });
  if (!replacements.length) {
    return false;
  }
  let tr = view.state.tr;
  replacements
    .sort((a, b) => b.from - a.from)
    .forEach((replacement) => {
      tr = tr.replaceWith(replacement.from, replacement.to, replacement.node);
    });
  if (tr.docChanged) {
    view.dispatch(tr);
    return true;
  }
  return false;
}

function transformCalloutBlockquote(node) {
  if (!node || node.type !== "blockquote") {
    return null;
  }
  const children = Array.isArray(node.children) ? node.children : [];
  if (!children.length) {
    return null;
  }
  const marker = parseCalloutMarker(extractParagraphText(children[0]));
  if (!marker) {
    return null;
  }
  const nextChildren = [];
  if (marker.text) {
    nextChildren.push(createCalloutParagraph(marker.text));
  }
  for (let index = 1; index < children.length; index += 1) {
    nextChildren.push(children[index]);
  }
  if (!nextChildren.length) {
    nextChildren.push(createCalloutParagraph(""));
  }
  return {
    type: "callout",
    emoji: marker.emoji,
    children: nextChildren,
  };
}

function calloutRemarkPlugin() {
  return (tree) => {
    if (!tree || !Array.isArray(tree.children)) {
      return;
    }
    const visit = (parent) => {
      if (!parent || !Array.isArray(parent.children)) {
        return;
      }
      parent.children = parent.children.map((child) => {
        const transformed = transformCalloutBlockquote(child);
        if (transformed) {
          return transformed;
        }
        visit(child);
        return child;
      });
    };
    visit(tree);
  };
}

function extractVideoEmbedNode(node, parent) {
  const parentType =
    typeof parent === "string"
      ? parent
      : parent && typeof parent.type === "string"
        ? parent.type
        : "root";
  if (parentType !== "root") {
    return null;
  }
  if (!node || node.type !== "paragraph" || !Array.isArray(node.children)) {
    return null;
  }
  if (node.children.length !== 1) {
    return null;
  }
  const child = node.children[0];
  let urlValue = "";
  if (child.type === "link" && typeof child.url === "string") {
    urlValue = child.url;
  } else if (child.type === "text" && typeof child.value === "string") {
    urlValue = child.value;
  }
  if (!urlValue) {
    return null;
  }
  const embed = parseVideoEmbedSource(urlValue.trim());
  if (!embed) {
    return null;
  }
  return {
    type: "video_embed",
    platform: embed.platform,
    source: embed.source,
    embed: embed.embed,
    aspect: embed.aspect,
  };
}

function videoEmbedRemarkPlugin() {
  return (tree) => {
    if (!tree || !Array.isArray(tree.children)) {
      return;
    }
    const visit = (parent) => {
      if (!parent || !Array.isArray(parent.children)) {
        return;
      }
      parent.children = parent.children.map((child) => {
        const transformed = extractVideoEmbedNode(child, parent);
        if (transformed) {
          return transformed;
        }
        visit(child);
        return child;
      });
    };
    visit(tree);
  };
}

const videoEmbedSchema = $nodeSchema("video_embed", () => ({
  group: "block",
  atom: true,
  isolating: true,
  selectable: true,
  draggable: true,
  attrs: {
    platform: { default: "" },
    source: { default: "" },
    embed: { default: "" },
    aspect: { default: VIDEO_EMBED_PLATFORMS.youtube.aspect },
  },
  parseDOM: [
    {
      tag: "div[data-video-embed]",
      getAttrs: (dom) => {
        if (!(dom instanceof HTMLElement)) {
          return false;
        }
        const iframe = dom.querySelector("iframe");
        return {
          platform: dom.getAttribute("data-video-platform") || "",
          source: dom.getAttribute("data-video-source") || "",
          embed:
            dom.getAttribute("data-video-embed-src") ||
            iframe?.getAttribute("src") ||
            "",
          aspect:
            dom.getAttribute("data-video-aspect") ||
            VIDEO_EMBED_PLATFORMS.youtube.aspect,
        };
      },
    },
  ],
  toDOM: (node) => {
    const platform = node?.attrs?.platform || "";
    const source = node?.attrs?.source || "";
    const embed =
      node?.attrs?.embed ||
      node?.attrs?.source ||
      (typeof source === "string" ? source : "");
    const aspect =
      node?.attrs?.aspect ||
      VIDEO_EMBED_PLATFORMS[platform]?.aspect ||
      VIDEO_EMBED_PLATFORMS.youtube.aspect;
    const title = buildVideoEmbedTitle(platform);
    const sandbox = videoEmbedSandboxForPlatform(platform);
    const iframeAttrs = {
      src: embed,
      title,
      loading: "lazy",
      allow: VIDEO_EMBED_ALLOW,
      allowfullscreen: "true",
      frameborder: "0",
      referrerpolicy: "strict-origin-when-cross-origin",
    };
    if (sandbox) {
      iframeAttrs.sandbox = sandbox;
    }
    return [
      "div",
      {
        "data-video-embed": "true",
        "data-video-platform": platform,
        "data-video-aspect": aspect,
        "data-video-source": source,
        "data-video-embed-src": embed,
        class: `video-embed ${VIDEO_EMBED_LOADING_CLASS}`,
        contenteditable: "false",
      },
      [
        "iframe",
        iframeAttrs,
      ],
    ];
  },
  parseMarkdown: {
    match: ({ type }) => type === "video_embed",
    runner: (state, node, type) => {
      state.openNode(type, {
        platform: node?.platform || "",
        source: node?.source || "",
        embed: node?.embed || "",
        aspect: node?.aspect || VIDEO_EMBED_PLATFORMS.youtube.aspect,
      });
      state.closeNode();
    },
  },
  toMarkdown: {
    match: (node) => node.type.name === "video_embed",
    runner: (state, node) => {
      const source =
        node?.attrs?.source ||
        node?.attrs?.embed ||
        node?.attrs?.src ||
        "";
      if (!source) {
        return;
      }
      state.openNode("paragraph");
      state.addNode("text", undefined, source);
      state.closeNode();
    },
  },
}));

const videoEmbedRemark = $remark("video-embed", () => videoEmbedRemarkPlugin());

const calloutSchema = $nodeSchema("callout", () => ({
  content: "block+",
  group: "block",
  defining: true,
  isolating: true,
  attrs: {
    emoji: { default: "" },
  },
  parseDOM: [
    {
      tag: "div[data-callout]",
      getAttrs: (dom) => {
        if (!(dom instanceof HTMLElement)) {
          return false;
        }
        return {
          emoji: normalizeCalloutEmoji(dom.getAttribute("data-emoji") || ""),
        };
      },
    },
  ],
  toDOM: (node) => {
    const emoji = normalizeCalloutEmoji(node?.attrs?.emoji);
    return [
      "div",
      {
        "data-callout": "true",
        "data-emoji": emoji,
        class: "callout-block",
      },
      ["div", { class: "callout-emoji" }, emoji || ""],
      ["div", { class: "callout-content" }, 0],
    ];
  },
  parseMarkdown: {
    match: ({ type }) => type === "callout",
    runner: (state, node, type) => {
      const emoji = normalizeCalloutEmoji(node?.emoji);
      state.openNode(type, { emoji });
      if (Array.isArray(node.children)) {
        state.next(node.children);
      }
      state.closeNode();
    },
  },
  toMarkdown: {
    match: (node) => node.type.name === "callout",
    runner: (state, node) => {
      const marker = buildCalloutMarker(node?.attrs?.emoji);
      state.openNode("blockquote");
      state.openNode("paragraph");
      state.addNode("text", undefined, marker);
      state.closeNode();
      state.next(node.content);
      state.closeNode();
    },
  },
}));

const calloutRemark = $remark("callout", () => calloutRemarkPlugin());

const calloutView = $view(calloutSchema.node, () => (node, view, getPos) => {
  let currentNode = node;
  const dom = document.createElement("div");
  dom.className = "callout-block";
  dom.dataset.callout = "true";

  const emojiWrapper = document.createElement("div");
  emojiWrapper.className = "callout-emoji-wrapper";

  const emojiButton = document.createElement("button");
  emojiButton.type = "button";
  emojiButton.className = "callout-emoji-button";
  emojiButton.setAttribute("aria-label", "选择表情");
  const emojiIcon = document.createElement("span");
  emojiIcon.className = "callout-emoji-icon";
  emojiButton.appendChild(emojiIcon);

  const emojiPanel = document.createElement("div");
  emojiPanel.className = "callout-emoji-panel";
  emojiPanel.dataset.open = "false";

  const emojiGrid = document.createElement("div");
  emojiGrid.className = "callout-emoji-grid";
  EMOJI_SLASH_ITEMS.forEach((item) => {
    if (!item || !item.emoji) {
      return;
    }
    const option = document.createElement("button");
    option.type = "button";
    option.className = "callout-emoji-option";
    option.textContent = item.emoji;
    option.setAttribute(
      "aria-label",
      typeof item.label === "string" ? item.label : item.emoji,
    );
    option.addEventListener("click", (event) => {
      event.preventDefault();
      event.stopPropagation();
      setEmoji(item.emoji);
      setPanelOpen(false);
    });
    emojiGrid.appendChild(option);
  });

  const removeButton = document.createElement("button");
  removeButton.type = "button";
  removeButton.className = "callout-emoji-remove";
  removeButton.textContent = "移除表情";
  removeButton.addEventListener("click", (event) => {
    event.preventDefault();
    event.stopPropagation();
    setEmoji("");
    setPanelOpen(false);
  });

  emojiPanel.appendChild(emojiGrid);
  emojiPanel.appendChild(removeButton);
  emojiWrapper.appendChild(emojiButton);
  emojiWrapper.appendChild(emojiPanel);

  const contentDOM = document.createElement("div");
  contentDOM.className = "callout-content";

  dom.appendChild(emojiWrapper);
  dom.appendChild(contentDOM);

  const resolveEditable = () => {
    if (!view) {
      return false;
    }
    if (typeof view.editable === "function") {
      return view.editable();
    }
    if (typeof view.editable === "boolean") {
      return view.editable;
    }
    return true;
  };

  let editable = resolveEditable();

  const renderEmoji = () => {
    const emoji = normalizeCalloutEmoji(currentNode?.attrs?.emoji);
    emojiIcon.textContent = emoji || "＋";
    emojiButton.dataset.empty = emoji ? "false" : "true";
    dom.dataset.emoji = emoji;
  };

  const applyEditable = () => {
    editable = resolveEditable();
    dom.dataset.editable = editable ? "true" : "false";
    emojiButton.disabled = !editable;
    emojiButton.setAttribute("aria-disabled", editable ? "false" : "true");
    if (!editable) {
      setPanelOpen(false);
    }
  };

  const setPanelOpen = (open) => {
    emojiPanel.dataset.open = open ? "true" : "false";
    emojiButton.setAttribute("aria-expanded", open ? "true" : "false");
  };

  const setEmoji = (nextEmoji) => {
    if (!editable || !view || !view.state) {
      return;
    }
    if (typeof getPos !== "function") {
      return;
    }
    const pos = getPos();
    if (typeof pos !== "number") {
      return;
    }
    const emoji = normalizeCalloutEmoji(nextEmoji);
    if (emoji === currentNode?.attrs?.emoji) {
      return;
    }
    const attrs = { ...currentNode.attrs, emoji };
    const tr = view.state.tr.setNodeMarkup(pos, undefined, attrs);
    view.dispatch(tr);
  };

  const handleButtonClick = (event) => {
    if (!editable) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    setPanelOpen(emojiPanel.dataset.open !== "true");
  };

  const handleDocumentClick = (event) => {
    if (!editable) {
      return;
    }
    if (!dom.contains(event.target)) {
      setPanelOpen(false);
    }
  };

  emojiButton.addEventListener("click", handleButtonClick);
  document.addEventListener("click", handleDocumentClick);

  renderEmoji();
  applyEditable();

  return {
    dom,
    contentDOM,
    update: (updatedNode) => {
      if (updatedNode.type !== currentNode.type) {
        return false;
      }
      currentNode = updatedNode;
      renderEmoji();
      applyEditable();
      return true;
    },
    destroy: () => {
      document.removeEventListener("click", handleDocumentClick);
    },
    stopEvent: (event) => {
      const target = event?.target;
      if (!(target instanceof HTMLElement)) {
        return false;
      }
      return Boolean(
        target.closest(".callout-emoji-button") ||
        target.closest(".callout-emoji-panel"),
      );
    },
  };
});

function insertEmojiFromSlash(ctx, emoji) {
  if (!ctx || !emoji) {
    return;
  }
  try {
    const view = ctx.get(editorViewCtx);
    if (!view) {
      return;
    }
    const { state } = view;
    if (!state) {
      return;
    }
    const { selection } = state;
    if (!selection) {
      return;
    }
    const { from, to } = selection;
    let start = from;
    try {
      const $from = selection.$from;
      const parent = $from.parent;
      const offset = $from.parentOffset;
      if (parent) {
        let textBefore = "";
        if (typeof parent.textBetween === "function") {
          textBefore = parent.textBetween(0, offset, "\n", "\n");
        } else if (typeof parent.textContent === "string") {
          textBefore = parent.textContent.slice(0, offset);
        }
        const match = textBefore.match(/\/[^\s]*$/);
        if (match && match[0]) {
          const commandLength = match[0].length;
          start = Math.max(from - commandLength, 0);
        }
      }
    } catch (error) {
      console.warn("[milkdown] Emoji 匹配命令失败", error);
    }
    const insertContent = `${emoji} `;
    const transaction = state.tr
      .insertText(insertContent, start, to)
      .scrollIntoView();
    view.dispatch(transaction);
    view.focus();
  } catch (error) {
    console.warn("[milkdown] Emoji 插入失败", error);
  }
}

function insertCalloutFromSlash(ctx) {
  if (!ctx) {
    return;
  }
  try {
    const view = ctx.get(editorViewCtx);
    if (!view) {
      return;
    }
    const { state } = view;
    if (!state) {
      return;
    }
    const { selection } = state;
    if (!selection) {
      return;
    }
    const { from, to } = selection;
    let start = from;
    try {
      const $from = selection.$from;
      const parent = $from.parent;
      const offset = $from.parentOffset;
      if (parent) {
        let textBefore = "";
        if (typeof parent.textBetween === "function") {
          textBefore = parent.textBetween(0, offset, "\n", "\n");
        } else if (typeof parent.textContent === "string") {
          textBefore = parent.textContent.slice(0, offset);
        }
        const match = textBefore.match(/\/[^\s]*$/);
        if (match && match[0]) {
          const commandLength = match[0].length;
          start = Math.max(from - commandLength, 0);
        }
      }
    } catch (error) {
      console.warn("[milkdown] Callout 匹配命令失败", error);
    }

    const calloutType = state.schema?.nodes?.callout;
    if (!calloutType) {
      return;
    }
    const paragraphType = state.schema?.nodes?.paragraph;
    let paragraph = null;
    if (paragraphType) {
      if (typeof paragraphType.createAndFill === "function") {
        paragraph = paragraphType.createAndFill();
      }
      if (!paragraph) {
        paragraph = paragraphType.create(null);
      }
    }
    const content = paragraph ? [paragraph] : undefined;
    const calloutNode = calloutType.create(
      { emoji: CALLOUT_DEFAULT_EMOJI },
      content,
    );
    if (!calloutNode) {
      return;
    }
    let transaction = state.tr.replaceRangeWith(start, to, calloutNode);
    if (TextSelection && typeof TextSelection.near === "function") {
      const selectionPos = Math.min(start + 1, transaction.doc.content.size);
      const resolved = transaction.doc.resolve(selectionPos);
      transaction = transaction.setSelection(TextSelection.near(resolved));
    }
    view.dispatch(transaction.scrollIntoView());
    view.focus();
  } catch (error) {
    console.warn("[milkdown] Callout 插入失败", error);
  }
}

function registerEmojiSlashMenu(builder) {
  if (!builder || typeof builder.addGroup !== "function") {
    return;
  }
  let groupInstance = null;
  try {
    groupInstance = builder.addGroup("emoji", "Emoji 表情");
  } catch (error) {
    try {
      groupInstance = builder.getGroup("emoji");
    } catch (innerError) {
      console.warn("[milkdown] Emoji 分组初始化失败", innerError);
    }
  }
  if (!groupInstance || typeof groupInstance.addItem !== "function") {
    return;
  }
  EMOJI_SLASH_ITEMS.forEach((item) => {
    if (!item || !item.key || !item.emoji) {
      return;
    }
    const label =
      typeof item.label === "string" && item.label.trim().length > 0
        ? item.label
        : `${item.emoji} Emoji`;
    groupInstance.addItem(`emoji-${item.key}`, {
      label,
      icon: item.emoji,
      onRun: (ctx) => insertEmojiFromSlash(ctx, item.emoji),
    });
  });
}

function registerCalloutSlashMenu(builder) {
  if (!builder || typeof builder.addGroup !== "function") {
    return;
  }
  let groupInstance = null;
  try {
    groupInstance = builder.addGroup("callout", "高亮块");
  } catch (error) {
    try {
      groupInstance = builder.getGroup("callout");
    } catch (innerError) {
      console.warn("[milkdown] Callout 分组初始化失败", innerError);
    }
  }
  if (!groupInstance || typeof groupInstance.addItem !== "function") {
    return;
  }
  groupInstance.addItem("callout", {
    label: "Callout 高亮块 /callout highlight",
    icon: CALLOUT_DEFAULT_EMOJI,
    onRun: (ctx) => insertCalloutFromSlash(ctx),
  });
}

function readInlineSelection(editor, controller) {
  if (!editor || typeof editor.action !== "function") {
    return null;
  }
  let info = null;
  editor.action((ctx) => {
    const view = ctx.get(editorViewCtx);
    if (!view) {
      return;
    }
    const { state } = view;
    const selection = state.selection;
    if (!selection || selection.empty) {
      return;
    }
    const { from, to } = selection;
    if (from === to) {
      return;
    }
    const raw = state.doc.textBetween(from, to, "\n", "\n");
    const normalized = normalizeSelectionContent(raw);
    if (!normalized.trim()) {
      return;
    }

    let coords = null;
    try {
      const start = view.coordsAtPos(from);
      const end = view.coordsAtPos(Math.max(from, to - 1));
      const left = Math.min(start.left, end.left);
      const right = Math.max(start.right, end.right);
      const top = Math.min(start.top, end.top);
      const bottom = Math.max(start.bottom, end.bottom);
      coords = {
        left,
        right,
        top,
        bottom,
        width: right - left,
        height: bottom - top,
      };
    } catch (error) {
      coords = null;
    }

    let contextText = normalized;
    try {
      const range = selection.$from?.blockRange(selection.$to);
      if (range) {
        const block = normalizeSelectionContent(
          state.doc.textBetween(range.start, range.end, "\n", "\n"),
        );
        if (block.trim()) {
          contextText = block;
        }
      }
    } catch (error) {
      // ignore block range issues
    }

    let revision = 0;
    if (controller && typeof controller === "object") {
      const rawRevision = controller.autoSaveRevision;
      if (Number.isFinite(rawRevision)) {
        revision = Number(rawRevision);
      } else if (typeof rawRevision === "string") {
        const parsed = Number(rawRevision);
        if (Number.isFinite(parsed)) {
          revision = parsed;
        }
      }
    }

    info = {
      from,
      to,
      text: raw,
      normalizedText: normalized,
      context: contextText,
      coords,
      revision,
      docSize: state.doc.content.size,
    };
  });
  return info;
}

function applyInlineAIResult(editor, payload = {}) {
  if (!editor || typeof editor.action !== "function") {
    throw new Error("编辑器尚未就绪");
  }
  const { from, to, expected, replacement } = payload;
  let applied = false;
  let failure = null;
  editor.action((ctx) => {
    const view = ctx.get(editorViewCtx);
    if (!view) {
      failure = new Error("编辑器尚未就绪");
      return;
    }
    const { state } = view;
    const docSize = state.doc.content.size;
    const start = clamp(from, 0, docSize);
    const end = clamp(to, 0, docSize);
    if (start === end) {
      failure = new Error("无法定位原始选区，请重新选择后再试");
      return;
    }
    const current = normalizeSelectionContent(
      state.doc.textBetween(start, end, "\n", "\n"),
    );
    const expectedNormalized = normalizeSelectionContent(expected || "");
    if (
      expectedNormalized.trim() &&
      current.trim() &&
      expectedNormalized.trim() !== current.trim()
    ) {
      failure = new Error("选中内容已发生变化，请重新选择后再试");
      return;
    }
    const text = typeof replacement === "string" ? replacement : "";
    const tr = state.tr.insertText(text, start, end);
    view.dispatch(tr.scrollIntoView());
    applied = true;
  });
  if (failure) {
    throw failure;
  }
  return applied;
}

function setupInlineAI(crepe, toast, getController) {
  const editor = crepe?.editor;
  if (!editor || typeof editor.action !== "function") {
    inlineAIToolbarHandler = null;
    return null;
  }

  let latestInfo = null;
  const notify =
    typeof toast === "function"
      ? toast
      : ({ type, message }) => {
          const prefix = type === "error" ? "[error]" : "[warn]";
          console.warn(`${prefix} ${message}`);
        };

  const hide = () => {
    if (
      typeof window === "undefined" ||
      typeof window.getSelection !== "function"
    ) {
      return;
    }
    try {
      const selection = window.getSelection();
      if (selection && typeof selection.removeAllRanges === "function") {
        selection.removeAllRanges();
      }
    } catch (error) {
      console.warn("[milkdown] 清理选区失败", error);
    }
  };

  const dispatchInlineAI = () => {
    const controller =
      typeof getController === "function" ? getController() : null;
    const info = readInlineSelection(editor, controller);
    latestInfo = info;
    if (!info || !info.normalizedText || !info.normalizedText.trim()) {
      notify({ type: "warning", message: "请选择需要改写的段落后再试" });
      hide();
      return;
    }
    window.dispatchEvent(
      new CustomEvent("post-editor:inline-ai", {
        detail: { selection: info },
      }),
    );
    hide();
  };

  inlineAIToolbarHandler = () => {
    try {
      dispatchInlineAI();
    } catch (error) {
      console.warn("[milkdown] AI Chat 工具触发失败", error);
    }
  };

  const destroy = () => {
    inlineAIToolbarHandler = null;
  };

  return {
    hide,
    destroy,
    getLatestSelection: () => latestInfo,
    readSelection: () => {
      const controller =
        typeof getController === "function" ? getController() : null;
      const info = readInlineSelection(editor, controller);
      latestInfo = info;
      return info;
    },
    applyChange: (options) => applyInlineAIResult(editor, options),
  };
}

function usePlugins(editor, plugins) {
  if (!editor || !plugins) {
    return;
  }
  plugins.forEach((plugin) => {
    if (!plugin) {
      return;
    }
    if (Array.isArray(plugin)) {
      usePlugins(editor, plugin);
      return;
    }
    if (typeof editor.use === "function") {
      editor.use(plugin);
    }
  });
}

async function uploadImageViaAPI(file) {
  const formData = new FormData();
  formData.append("image", file);

  let response;
  try {
    response = await fetch("/admin/api/upload/image", {
      method: "POST",
      body: formData,
    });
  } catch (error) {
    const message = error?.message || "网络异常，图片上传失败";
    throw new Error(message);
  }

  let payload = null;
  try {
    payload = await response.json();
  } catch (error) {
    console.error("[milkdown] 图片上传响应解析失败", error);
  }

  if (!response.ok || !payload || payload.success !== 1) {
    const message =
      payload?.error ||
      payload?.message ||
      `图片上传失败（${response?.status ?? "未知状态"}）`;
    throw new Error(message);
  }

  const url = payload?.data?.url || payload?.data?.filePath;
  if (!url) {
    throw new Error("图片上传成功但未返回访问地址");
  }
  return url;
}

function applyMilkdownPlugins(editor, toast) {
  if (!editor || typeof editor.config !== "function") {
    return;
  }

  const reAlignSlashMenu = (menu) => {
    try {
      if (!menu || !editor || typeof editor.action !== "function") {
        return;
      }
      editor.action((ctx) => {
        const view = ctx.get(editorViewCtx);
        if (!view) {
          return;
        }
        const { selection } = view.state;
        const pos = typeof selection?.to === "number" ? selection.to : 0;
        const coords = view.coordsAtPos(Math.max(pos, 0));
        const parent = menu.offsetParent || menu.parentElement || document.body;
        const parentRect = parent.getBoundingClientRect();
        const scrollLeft =
          typeof parent.scrollLeft === "number"
            ? parent.scrollLeft
            : window.scrollX;
        const scrollTop =
          typeof parent.scrollTop === "number"
            ? parent.scrollTop
            : window.scrollY;
        const top = (coords?.bottom ?? 0) - parentRect.top + scrollTop + 10;
        const left = (coords?.left ?? 0) - parentRect.left + scrollLeft;
        menu.style.top = `${top}px`;
        menu.style.bottom = "auto";
        menu.style.left = `${left}px`;
      });
    } catch (error) {
      console.warn("[milkdown] Slash 菜单定位失败", error);
    }
  };

  const ensureSlashMenuPlacement = () => {
    const loopState = {
      id: null,
      observer: null,
    };

    const stopLoop = () => {
      if (loopState.id) {
        cancelAnimationFrame(loopState.id);
        loopState.id = null;
      }
    };

    const watchMenu = (menu) => {
      if (!menu) {
        return;
      }
      // data-show 为 true 时重复执行对齐，避免浮动库自动翻转
      const tick = () => {
        if (!menu || menu.dataset.show === "false") {
          stopLoop();
          return;
        }
        reAlignSlashMenu(menu);
        loopState.id = requestAnimationFrame(tick);
      };
      stopLoop();
      loopState.id = requestAnimationFrame(tick);

      if (loopState.observer) {
        loopState.observer.disconnect();
      }
      loopState.observer = new MutationObserver((mutations) => {
        for (const mutation of mutations) {
          if (
            mutation.type === "attributes" &&
            mutation.attributeName === "data-show"
          ) {
            if (menu.dataset.show !== "false") {
              stopLoop();
              loopState.id = requestAnimationFrame(tick);
            } else {
              stopLoop();
            }
          }
        }
      });
      loopState.observer.observe(menu, {
        attributes: true,
        attributeFilter: ["data-show"],
      });
    };

    const attachWhenReady = () => {
      const menu = document.querySelector(".milkdown-slash-menu");
      if (menu) {
        watchMenu(menu);
        return true;
      }
      return false;
    };

    if (attachWhenReady()) {
      return;
    }

    const finder = setInterval(() => {
      if (attachWhenReady()) {
        clearInterval(finder);
      }
    }, 200);
    setTimeout(() => clearInterval(finder), 4000);
  };

  editor.config((ctx) => {
    ctx.update(uploadConfig.key, (prev) => {
      const fallbackUploader =
        prev && typeof prev.uploader === "function" ? prev.uploader : null;
      return {
        ...prev,
        enableHtmlFileUploader: true,
        uploader: async (files, schema) => {
          const imageNode = schema?.nodes?.image;
          const items = files
            ? Array.from(files).filter(
                (file) => file && file.type && file.type.startsWith("image/"),
              )
            : [];

          if (!items.length || !imageNode) {
            return fallbackUploader ? fallbackUploader(files, schema) : [];
          }

          const createdNodes = [];
          for (const file of items) {
            try {
              const url = await uploadImageViaAPI(file);
              const node = imageNode.createAndFill({
                src: url,
                alt: file.name || "",
              });
              if (node) {
                createdNodes.push(node);
              }
            } catch (error) {
              const message = error?.message || "图片上传失败";
              if (typeof toast === "function") {
                toast({ type: "error", message });
              } else {
                console.error("[milkdown] 图片上传失败", error);
              }
            }
          }

          if (!createdNodes.length) {
            return fallbackUploader ? fallbackUploader(files, schema) : [];
          }

          return createdNodes;
        },
      };
    });
  });

  editor.config((ctx) => {
    ctx.update(editorViewOptionsCtx, (prev) => {
      const previous = prev || {};
      const previousHandlePaste =
        typeof previous.handlePaste === "function"
          ? previous.handlePaste
          : null;
      const handlePaste = (view, event, slice) => {
        if (handleVideoEmbedPaste(view, event)) {
          return true;
        }
        if (handleMarkdownTablePaste(view, event)) {
          return true;
        }
        if (previousHandlePaste) {
          return previousHandlePaste(view, event, slice);
        }
        return false;
      };
      return {
        ...previous,
        handlePaste,
      };
    });
  });

  usePlugins(editor, [upload, cursor]);
  usePlugins(editor, [
    videoEmbedSchema,
    videoEmbedRemark,
    calloutSchema,
    calloutRemark,
    calloutView,
  ]);
  usePlugins(editor, [math]);
  if (typeof window !== "undefined") {
    ensureSlashMenuPlacement();
  }
}

class PostDraftController {
  constructor(crepe, initialData = {}) {
    this.crepe = crepe;
    this.editor = crepe?.editor ?? null;
    this.getMarkdown =
      typeof crepe?.getMarkdown === "function"
        ? () => crepe.getMarkdown()
        : () => "";
    this.toast = createToast();
    this.initialData = initialData || {};
    this.postData = this.normalizePost(initialData.post);
    this.latestPublication = initialData.latestPublication || null;
    const sessionCandidate = coerceString(
      pickProperty(initialData, ["draft_session_id", "draftSessionId"], ""),
      "",
    ).trim();
    this.draftSessionId = sessionCandidate || generateDraftSessionId();
    this.postId = this.resolvePostId(this.postData);
    this.eventTarget = typeof window !== "undefined" ? window : null;
    this.loading = false;
    this.autoSaving = false;
    this.autoSavePending = false;
    this.autoSaveError = "";
    this.autoSaveRevision = 0;
    this.lastAutoSavedAt = null;
    this.publishing = false;
    this.autoSaveIntervalId = null;
    this.changeMonitorId = null;
    this.visibilityHandler = null;
    this.unloadHandler = null;
    this.boundKeyHandler = this.handleKeydown.bind(this);
    this.pendingAutoSaveFlush = false;
    this.contentMetrics = { words: 0, characters: 0, paragraphs: 0 };

    const initialContent = this.safeGetInitialContent();
    this.currentContent = initialContent;
    this.lastSavedContent = initialContent;
    this.updateContentMetrics(initialContent);
  }

  getPostSnapshot() {
    if (!this.postData || typeof this.postData !== "object") {
      return {};
    }
    return cloneValue(this.postData) || {};
  }

  emitPostEvent(type, detail = {}) {
    if (
      !this.eventTarget ||
      typeof this.eventTarget.dispatchEvent !== "function" ||
      !type
    ) {
      return;
    }
    const payload = {
      controller: this,
      post: this.getPostSnapshot(),
      ...detail,
    };
    try {
      this.eventTarget.dispatchEvent(
        new CustomEvent(`post-editor:${type}`, { detail: payload }),
      );
    } catch (error) {
      console.warn("[milkdown] 分发事件失败", type, error);
    }
  }

  notifyPostChange(reason = "") {
    const detail = reason ? { reason } : {};
    this.emitPostEvent("post-updated", detail);
  }

  updateContentMetrics(markdown) {
    const metrics = calculateContentMetrics(markdown);
    this.contentMetrics = metrics;
    this.emitPostEvent("metrics", { metrics });
    return metrics;
  }

  normalizePost(post) {
    const source = post && typeof post === "object" ? { ...post } : {};
    if (source.Title === undefined && typeof source.title === "string") {
      source.Title = source.title;
    }
    if (source.Summary === undefined && typeof source.summary === "string") {
      source.Summary = source.summary;
    }
    if (source.Content === undefined && typeof source.content === "string") {
      source.Content = source.content;
    }
    if (source.CoverURL === undefined && typeof source.cover_url === "string") {
      source.CoverURL = source.cover_url;
    }
    if (source.CoverWidth === undefined && source.cover_width !== undefined) {
      source.CoverWidth = source.cover_width;
    }
    if (source.CoverHeight === undefined && source.cover_height !== undefined) {
      source.CoverHeight = source.cover_height;
    }
    if (!Array.isArray(source.Tags) && Array.isArray(source.tags)) {
      source.Tags = source.tags;
    }
    if (source.Status === undefined && typeof source.status === "string") {
      source.Status = source.status;
    }
    if (source.UserID === undefined && source.user_id !== undefined) {
      source.UserID = source.user_id;
    }

    const normalizedTags = this.normalizeTags(source.Tags);
    const primaryTags = normalizedTags.map(
      (tag) => cloneValue(tag) || { ...tag },
    );
    source.Tags = primaryTags;
    source.tags = primaryTags.map((tag) => cloneValue(tag) || { ...tag });

    return {
      Title: "",
      Summary: "",
      Content: "",
      Status: "draft",
      Tags: [],
      CoverURL: "",
      CoverWidth: 0,
      CoverHeight: 0,
      ...source,
    };
  }

  normalizeTag(tag) {
    if (tag === null || tag === undefined) {
      return null;
    }
    let source = {};
    if (typeof tag === "object") {
      source = cloneValue(tag) || { ...tag };
    } else {
      const numeric = coerceNumber(tag, NaN);
      if (!Number.isFinite(numeric)) {
        return null;
      }
      source = { ID: numeric };
    }
    const idCandidate = pickProperty(source, ["ID", "id", "Id"], null);
    const id = coerceNumber(idCandidate, NaN);
    if (!Number.isFinite(id)) {
      return null;
    }
    const nameCandidate = coerceString(
      pickProperty(source, ["Name", "name"], ""),
      "",
    );
    const slugCandidate = coerceString(
      pickProperty(source, ["Slug", "slug"], ""),
      "",
    );
    const name = nameCandidate.trim();
    const slug = slugCandidate.trim();
    const normalized = {
      ...source,
      ID: id,
      id,
      Name: name,
      name,
    };
    if (slug) {
      normalized.Slug = slug;
      normalized.slug = slug;
    } else {
      delete normalized.Slug;
      delete normalized.slug;
    }
    return normalized;
  }

  normalizeTags(tags) {
    if (!Array.isArray(tags)) {
      return [];
    }
    const normalized = [];
    const seen = new Set();
    for (const tag of tags) {
      const normalizedTag = this.normalizeTag(tag);
      if (!normalizedTag) {
        continue;
      }
      const id = this.getTagId(normalizedTag);
      if (id === null || seen.has(id)) {
        continue;
      }
      seen.add(id);
      normalized.push(normalizedTag);
    }
    return normalized;
  }

  getTagId(tag) {
    if (tag && typeof tag === "object") {
      const value = pickProperty(tag, ["ID", "id", "Id"], null);
      const numeric = coerceNumber(value, NaN);
      return Number.isFinite(numeric) ? numeric : null;
    }
    const numeric = coerceNumber(tag, NaN);
    return Number.isFinite(numeric) ? numeric : null;
  }

  tagIdSequence(tags) {
    if (!Array.isArray(tags)) {
      return [];
    }
    return tags.map((tag) => this.getTagId(tag)).filter((id) => id !== null);
  }

  resolveTagsFromIds(ids, availableTags = []) {
    const raw = Array.isArray(ids) ? ids : [];
    const uniqueIds = [];
    for (const value of raw) {
      const id = this.getTagId(value);
      if (id === null || uniqueIds.includes(id)) {
        continue;
      }
      uniqueIds.push(id);
    }
    const available = this.normalizeTags(availableTags);
    const existing = this.normalizeTags(this.postData?.Tags);
    const lookup = [...available, ...existing];
    const resolved = [];
    for (const id of uniqueIds) {
      const match = lookup.find((tag) => this.getTagId(tag) === id);
      if (match) {
        resolved.push(match);
      }
    }
    return resolved;
  }

  setTags(tags) {
    const normalized = this.normalizeTags(tags);
    const previous = this.tagIdSequence(this.postData?.Tags);
    const next = this.tagIdSequence(normalized);
    const sameSelection =
      previous.length === next.length &&
      previous.every((id, index) => id === next[index]);
    const clones = normalized.map((tag) => cloneValue(tag) || { ...tag });
    this.postData.Tags = clones;
    this.postData.tags = clones.map((tag) => cloneValue(tag) || { ...tag });
    if (!sameSelection) {
      this.markDirty();
    }
    this.notifyPostChange("tags");
    return this.postData.Tags;
  }

  setTagIds(ids, availableTags = []) {
    const resolved = this.resolveTagsFromIds(ids, availableTags);
    return this.setTags(resolved);
  }

  resolvePostId(post) {
    if (!post || typeof post !== "object") {
      return "new";
    }
    const id = pickProperty(post, ["ID", "id", "Id"], null);
    if (id === null || id === undefined) {
      return "new";
    }
    const stringified = coerceString(id, "").trim();
    return stringified ? stringified : "new";
  }

  safeGetInitialContent() {
    const fromInitial =
      typeof this.initialData.content === "string"
        ? this.initialData.content
        : "";
    if (fromInitial) {
      return fromInitial;
    }
    const fromPost = pickProperty(this.postData, ["Content", "content"], "");
    return typeof fromPost === "string" ? fromPost : "";
  }

  safeGetMarkdown() {
    try {
      const value = this.getMarkdown();
      return typeof value === "string" ? value : "";
    } catch (error) {
      console.error("[milkdown] 获取 Markdown 内容失败", error);
      return "";
    }
  }

  init() {
    this.startChangeMonitor();
    this.bindKeyboardShortcuts();
    this.initAutoSaveLifecycle();
  }

  dispose() {
    this.stopChangeMonitor();
    this.stopAutoSaveTimer();
    if (this.visibilityHandler) {
      document.removeEventListener("visibilitychange", this.visibilityHandler);
      this.visibilityHandler = null;
    }
    if (this.unloadHandler) {
      window.removeEventListener("pagehide", this.unloadHandler);
      window.removeEventListener("beforeunload", this.unloadHandler);
      this.unloadHandler = null;
    }
    if (this.boundKeyHandler) {
      window.removeEventListener("keydown", this.boundKeyHandler);
    }
  }

  startChangeMonitor() {
    this.stopChangeMonitor();
    this.changeMonitorId = window.setInterval(() => {
      const markdown = this.safeGetMarkdown();
      if (markdown === this.currentContent) {
        return;
      }
      const isInitialChange =
        this.currentContent === this.lastSavedContent &&
        markdown === this.lastSavedContent;
      this.currentContent = markdown;
      this.updateContentMetrics(markdown);
      if (isInitialChange || this.currentContent === this.lastSavedContent) {
        this.autoSavePending = false;
        return;
      }
      this.markDirty();
    }, DEFAULT_CHANGE_POLL_INTERVAL);
  }

  stopChangeMonitor() {
    if (this.changeMonitorId) {
      window.clearInterval(this.changeMonitorId);
      this.changeMonitorId = null;
    }
  }

  bindKeyboardShortcuts() {
    if (this.boundKeyHandler) {
      window.addEventListener("keydown", this.boundKeyHandler);
    }
  }

  toggleLinkToolbar() {
    if (!this.editor || typeof this.editor.action !== "function") {
      return false;
    }
    let toggled = false;
    try {
      this.editor.action((ctx) => {
        const view = ctx.get(editorViewCtx);
        if (!view) {
          return;
        }
        const isFocused =
          typeof view.hasFocus === "function"
            ? view.hasFocus()
            : !!(
                view.dom instanceof Element &&
                view.dom.contains(document.activeElement)
              );
        if (!isFocused) {
          return;
        }
        const state = view.state;
        if (!state || state.selection?.empty) {
          return;
        }
        const { from, to } = state.selection;
        if (typeof from !== "number" || typeof to !== "number" || from === to) {
          return;
        }
        const commands = ctx.get(commandsCtx);
        if (!commands || typeof commands.call !== "function") {
          return;
        }
        commands.call(toggleLinkCommand.key);
        toggled = true;
      });
    } catch (error) {
      console.warn("[milkdown] 链接工具栏切换失败", error);
    }
    return toggled;
  }

  handleKeydown(event) {
    if (!event || !(event.ctrlKey || event.metaKey)) {
      return;
    }
    const key = typeof event.key === "string" ? event.key.toLowerCase() : "";
    if (key === "s") {
      event.preventDefault();
      if (this.loading || this.autoSaving) {
        return;
      }
      void this.saveDraft({
        redirectOnCreate: false,
        silent: false,
        notifyOnSilent: true,
        useLoadingState: false,
      });
      return;
    }
    if (key === "k") {
      const toggled = this.toggleLinkToolbar();
      if (toggled) {
        event.preventDefault();
      }
    }
  }

  initAutoSaveLifecycle() {
    this.startAutoSaveTimer();
    if (!this.visibilityHandler) {
      this.visibilityHandler = () => {
        if (document.visibilityState === "hidden") {
          void this.autoSaveIfNeeded();
        }
      };
      document.addEventListener("visibilitychange", this.visibilityHandler);
    }
    if (!this.unloadHandler) {
      this.unloadHandler = () => {
        if (this.pendingAutoSaveFlush) {
          return;
        }
        this.pendingAutoSaveFlush = true;
        void this.autoSaveIfNeeded().finally(() => {
          this.pendingAutoSaveFlush = false;
        });
      };
      window.addEventListener("pagehide", this.unloadHandler);
      window.addEventListener("beforeunload", this.unloadHandler);
    }
  }

  startAutoSaveTimer() {
    this.stopAutoSaveTimer();
    this.autoSaveIntervalId = window.setInterval(() => {
      void this.autoSaveIfNeeded();
    }, DEFAULT_AUTOSAVE_INTERVAL);
  }

  stopAutoSaveTimer() {
    if (this.autoSaveIntervalId) {
      window.clearInterval(this.autoSaveIntervalId);
      this.autoSaveIntervalId = null;
    }
  }

  markDirty() {
    this.autoSaveRevision += 1;
    this.autoSavePending = true;
    this.autoSaveError = "";
    this.emitPostEvent("dirty", { revision: this.autoSaveRevision });
  }

  setTitle(value) {
    const normalized = coerceString(value, "").trim();
    const previous = coerceString(
      pickProperty(this.postData, ["Title", "title"], ""),
      "",
    ).trim();
    const markdown = this.safeGetMarkdown();
    const updatedMarkdown = synchronizeTitleHeading(markdown, normalized);
    const contentChanged = updatedMarkdown !== markdown;

    if (!contentChanged && previous === normalized) {
      return previous;
    }

    if (!this.postData || typeof this.postData !== "object") {
      this.postData = {};
    }

    if (contentChanged) {
      const applyResult =
        typeof this.setMarkdown === "function"
          ? this.setMarkdown(updatedMarkdown, { flush: true })
          : null;
      if (applyResult && typeof applyResult.then === "function") {
        applyResult.catch((error) =>
          console.warn("[milkdown] 更新标题时同步 Markdown 失败", error),
        );
      }
      this.postData.Content = updatedMarkdown;
      this.postData.content = updatedMarkdown;
      this.currentContent = updatedMarkdown;
      if (typeof this.updateContentMetrics === "function") {
        this.updateContentMetrics(updatedMarkdown);
      }
    }

    this.postData.Title = normalized;
    this.postData.title = normalized;
    this.markDirty();
    this.notifyPostChange("title");
    if (contentChanged) {
      this.notifyPostChange("content");
    }
    return normalized;
  }

  setSummary(value) {
    const normalized = coerceString(value, "");
    if (this.postData.Summary === normalized) {
      return this.postData.Summary;
    }
    this.postData.Summary = normalized;
    this.postData.summary = normalized;
    this.markDirty();
    this.notifyPostChange("summary");
    return normalized;
  }

  setCover(info = {}) {
    const nextUrl = coerceString(
      pickProperty(info, ["url", "CoverURL", "cover_url"], ""),
      "",
    ).trim();
    const nextWidth = coerceNumber(
      pickProperty(info, ["width", "CoverWidth", "cover_width"], 0),
      0,
    );
    const nextHeight = coerceNumber(
      pickProperty(info, ["height", "CoverHeight", "cover_height"], 0),
      0,
    );

    const sameAsBefore =
      this.postData.CoverURL === nextUrl &&
      this.postData.CoverWidth === nextWidth &&
      this.postData.CoverHeight === nextHeight;
    if (sameAsBefore) {
      return {
        url: this.postData.CoverURL,
        width: this.postData.CoverWidth,
        height: this.postData.CoverHeight,
      };
    }

    this.postData.CoverURL = nextUrl;
    this.postData.cover_url = nextUrl;
    this.postData.CoverWidth = nextWidth;
    this.postData.cover_width = nextWidth;
    this.postData.CoverHeight = nextHeight;
    this.postData.cover_height = nextHeight;
    this.markDirty();
    this.notifyPostChange("cover");
    return {
      url: nextUrl,
      width: nextWidth,
      height: nextHeight,
    };
  }

  clearCover() {
    return this.setCover({ url: "", width: 0, height: 0 });
  }

  async resetToLatestPublication(options = {}) {
    const { publication: overridePublication = null, silent = false } =
      options || {};
    const publication = overridePublication || this.latestPublication;
    if (!publication || typeof publication !== "object") {
      if (!silent) {
        this.toast({
          message: "当前没有线上版本可供恢复",
          type: "error",
        });
      }
      return false;
    }

    this.setLatestPublication(publication);
    const content = coerceString(
      pickProperty(publication, ["Content", "content"], ""),
      "",
    );
    const summary = coerceString(
      pickProperty(publication, ["Summary", "summary"], ""),
      "",
    );
    const coverUrl = coerceString(
      pickProperty(publication, ["CoverURL", "cover_url"], ""),
      "",
    ).trim();
    const coverWidth = coerceNumber(
      pickProperty(publication, ["CoverWidth", "cover_width"], 0),
      0,
    );
    const coverHeight = coerceNumber(
      pickProperty(publication, ["CoverHeight", "cover_height"], 0),
      0,
    );
    const tags = this.normalizeTags(
      pickProperty(publication, ["Tags", "tags"], []),
    );

    if (this.crepe && typeof this.crepe.setMarkdown === "function") {
      try {
        const applied = this.crepe.setMarkdown(content, {
          flush: true,
        });
        if (applied && typeof applied.then === "function") {
          await applied;
        }
      } catch (error) {
        console.warn("[milkdown] 重置草稿时同步 Markdown 失败", error);
      }
    }

    const clonedTags = tags.map((tag) => cloneValue(tag) || { ...tag });
    this.postData.Content = content;
    this.postData.content = content;
    this.postData.Summary = summary;
    this.postData.summary = summary;
    this.postData.CoverURL = coverUrl;
    this.postData.cover_url = coverUrl;
    this.postData.CoverWidth = coverWidth;
    this.postData.cover_width = coverWidth;
    this.postData.CoverHeight = coverHeight;
    this.postData.cover_height = coverHeight;
    this.postData.Tags = clonedTags;
    this.postData.tags = clonedTags.map((tag) => cloneValue(tag) || { ...tag });

    this.currentContent = content;
    this.updateContentMetrics(content);
    this.markDirty();
    this.notifyPostChange("reset-publication");
    return true;
  }

  buildPayload(content) {
    const post = this.postData || {};
    const markdown =
      typeof content === "string"
        ? content
        : coerceString(pickProperty(post, ["Content", "content"], ""), "");
    const derivedTitle = deriveTitleFromMarkdown(markdown);
    const fallbackTitle = coerceString(
      pickProperty(post, ["Title", "title"], ""),
      "",
    );
    const title = derivedTitle || fallbackTitle;
    if (post && typeof post === "object") {
      post.Title = title;
      post.title = title;
    }
    const summary = coerceString(
      pickProperty(post, ["Summary", "summary"], ""),
      "",
    );
    const coverUrl = coerceString(
      pickProperty(post, ["CoverURL", "cover_url"], ""),
      "",
    );
    const coverWidth = coerceNumber(
      pickProperty(post, ["CoverWidth", "cover_width"], 0),
      0,
    );
    const coverHeight = coerceNumber(
      pickProperty(post, ["CoverHeight", "cover_height"], 0),
      0,
    );
    const tags = Array.isArray(post.Tags)
      ? post.Tags
      : Array.isArray(post.tags)
        ? post.tags
        : [];
    const tagIds = tags
      .map((tag) => this.getTagId(tag))
      .filter((id) => id !== null);

    return {
      title,
      summary,
      content,
      tag_ids: tagIds,
      cover_url: coverUrl,
      cover_width: coverWidth,
      cover_height: coverHeight,
      draft_session_id: this.draftSessionId,
    };
  }

  updatePostData(nextPost) {
    if (!nextPost || typeof nextPost !== "object") {
      return;
    }
    const merged = { ...this.postData, ...nextPost };
    this.postData = this.normalizePost(merged);
    const resolvedId = this.resolvePostId(this.postData);
    if (resolvedId !== "new") {
      this.postId = resolvedId;
    }
    this.notifyPostChange("server");
  }

  setLatestPublication(publication) {
    if (!publication || typeof publication !== "object") {
      this.latestPublication = null;
      return this.latestPublication;
    }
    this.latestPublication =
      cloneValue(publication, { skipStructuredClone: true }) || publication;
    return this.latestPublication;
  }

  getLatestPublication() {
    return cloneValue(this.latestPublication, { skipStructuredClone: true });
  }

  async publish(options = {}) {
    const { silent = false, publishedAt = "" } = options;
    if (this.publishing) {
      return false;
    }

    const shouldSaveDraft = this.postId === "new" || this.autoSavePending;
    if (shouldSaveDraft) {
      const saved = await this.saveDraft({
        redirectOnCreate: false,
        silent: true,
        notifyOnSilent: false,
        useLoadingState: false,
      });
      if (!saved) {
        if (!silent) {
          this.toast({
            message: "请先完善内容并保存草稿后再发布",
            type: "warning",
          });
        }
        return false;
      }
    }

    if (this.postId === "new") {
      if (!silent) {
        this.toast({
          message: "文章尚未保存，无法发布",
          type: "warning",
        });
      }
      return false;
    }

    const payload = {};
    const normalizedPublishedAt = coerceString(publishedAt, "").trim();
    if (normalizedPublishedAt) {
      payload.published_at = normalizedPublishedAt;
    }

    this.publishing = true;
    try {
      const requestInit = { method: "POST" };
      if (Object.keys(payload).length > 0) {
        requestInit.headers = { "Content-Type": "application/json" };
        requestInit.body = JSON.stringify(payload);
      }
      const response = await fetch(
        `/admin/api/posts/${this.postId}/publish`,
        requestInit,
      );
      let data = {};
      try {
        data = await response.json();
      } catch (error) {
        data = {};
      }
      if (!response.ok) {
        const message = data?.error || data?.message || "发布失败，请稍后重试";
        if (message.includes("请完善标题与正文内容")) {
          const current = this.safeGetMarkdown();
          console.warn("[milkdown] 发布校验失败", {
            postId: this.postId,
            contentLength: current.length,
            derivedTitle: deriveTitleFromMarkdown(current),
          });
        }
        if (!silent) {
          this.toast({ message, type: "error" });
        }
        return false;
      }

      if (data?.post) {
        this.updatePostData(data.post);
      }
      if (data?.publication) {
        this.setLatestPublication(data.publication);
      }

      const shouldNotify = !silent;
      if (shouldNotify && Array.isArray(data?.notices)) {
        data.notices.forEach((message) => {
          this.toast({ message, type: "info" });
        });
      }
      if (shouldNotify && Array.isArray(data?.warnings)) {
        data.warnings.forEach((message) => {
          this.toast({ message, type: "warning" });
        });
      }
      if (shouldNotify && data?.message) {
        this.toast({ message: data.message, type: "success" });
      }

      this.lastSavedContent = this.currentContent;
      this.autoSavePending = false;
      this.autoSaveError = "";
      this.lastAutoSavedAt = new Date();

      this.notifyPostChange("publish");
      if (this.latestPublication) {
        this.emitPostEvent("publication", {
          publication: this.getLatestPublication(),
        });
      }
      return true;
    } catch (error) {
      const message = error?.message || "发布失败，请稍后重试";
      if (!silent) {
        this.toast({ message, type: "error" });
      }
      return false;
    } finally {
      this.publishing = false;
    }
  }

  async saveDraft(options = {}) {
    const {
      redirectOnCreate = false,
      silent = false,
      notifyOnSilent = false,
      useLoadingState = false,
      contentOverride = null,
    } = options;

    if (this.loading && useLoadingState) {
      return false;
    }

    const shouldToggleLoading = useLoadingState;
    if (shouldToggleLoading) {
      this.loading = true;
    }

    const previousContent = this.currentContent;
    const content =
      typeof contentOverride === "string"
        ? contentOverride
        : this.safeGetMarkdown();
    this.currentContent = content;
    this.postData.Content = content;

    const derivedTitle = deriveTitleFromMarkdown(content);
    if (this.postData && typeof this.postData === "object") {
      this.postData.Title = derivedTitle;
      this.postData.title = derivedTitle;
    }

    if (
      typeof previousContent === "string" &&
      previousContent.trim() &&
      !content.trim()
    ) {
      console.warn("[milkdown] 草稿内容意外为空", {
        postId: this.postId,
        derivedTitle,
        previousLength: previousContent.length,
      });
    }

    const hasMeaningfulContent = [
      pickProperty(this.postData, ["Title", "title"], ""),
      pickProperty(this.postData, ["Summary", "summary"], ""),
      content,
    ].some((value) => typeof value === "string" && value.trim().length > 0);

    if (!hasMeaningfulContent && this.postId === "new") {
      if (shouldToggleLoading) {
        this.loading = false;
      }
      return false;
    }

    const revisionAtStart = this.autoSaveRevision;
    const payload = this.buildPayload(content);

    let url = "/admin/api/posts";
    let method = "POST";
    if (this.postId !== "new") {
      url = `/admin/api/posts/${this.postId}`;
      method = "PUT";
    }

    try {
      const response = await fetch(url, {
        method,
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
      const data = await response.json();
      if (!response.ok) {
        const message = data.error || data.message || "保存失败，请稍后重试";
        if (silent && !notifyOnSilent) {
          this.autoSaveError = message;
        } else {
          this.toast({ message, type: "error" });
        }
        return false;
      }

      if (data.post) {
        this.updatePostData(data.post);
      }

      const shouldNotify = !silent || notifyOnSilent;
      if (shouldNotify && Array.isArray(data.notices)) {
        data.notices.forEach((message) => {
          this.toast({ message, type: "info" });
        });
      }
      if (shouldNotify && Array.isArray(data.warnings)) {
        data.warnings.forEach((message) => {
          this.toast({ message, type: "warning" });
        });
      }

      if (!silent && data.message) {
        this.toast({ message: data.message, type: "success" });
      }

      if (this.autoSaveRevision === revisionAtStart) {
        this.autoSavePending = false;
      }
      this.lastSavedContent = content;
      this.lastAutoSavedAt = new Date();
      this.autoSaveError = "";

      this.notifyPostChange("save");

      if (
        this.postId !== "new" &&
        window.location &&
        !window.location.pathname.includes("/edit")
      ) {
        const target = `/admin/posts/${this.postId}/edit`;
        if (redirectOnCreate) {
          window.location.href = target;
        } else {
          window.history.replaceState({}, "", target);
        }
      }

      return true;
    } catch (error) {
      const message = error?.message || "保存失败，请稍后重试";
      if (silent && !notifyOnSilent) {
        this.autoSaveError = message;
      } else {
        this.toast({ message, type: "error" });
      }
      return false;
    } finally {
      if (shouldToggleLoading) {
        this.loading = false;
      }
    }
  }

  async autoSaveIfNeeded() {
    if (this.autoSaving || this.loading) {
      return false;
    }
    if (!this.autoSavePending) {
      return false;
    }

    const content = this.safeGetMarkdown();
    this.currentContent = content;
    this.postData.Content = content;

    this.autoSaving = true;
    try {
      const saved = await this.saveDraft({
        redirectOnCreate: false,
        silent: true,
        notifyOnSilent: false,
        useLoadingState: false,
        contentOverride: content,
      });
      return saved;
    } finally {
      this.autoSaving = false;
    }
  }
}

async function initialize() {
  const mount = document.getElementById("milkdown-app");
  if (!mount) {
    console.warn("[milkdown] 未找到挂载节点 #milkdown-app");
    return;
  }

  ensureStyles();

  const initial = getInitialMarkdown();

  try {
    const toast = createToast();
    inlineAIToolbarHandler = null;

    const toolbarKey = Crepe?.Feature?.Toolbar ?? "toolbar";
    const blockEditKey = Crepe?.Feature?.BlockEdit ?? "block-edit";
    const imageBlockKey = Crepe?.Feature?.ImageBlock ?? "image-block";
    const imageUploader = async (file) => {
      if (!file) {
        toast({ message: "请选择要上传的图片", type: "warning" });
        throw new Error("请选择要上传的图片");
      }
      try {
        return await uploadImageViaAPI(file);
      } catch (error) {
        const message = error?.message || "图片上传失败";
        toast({ message, type: "error" });
        throw error;
      }
    };
    const featureConfigs = {
      [toolbarKey]: {
        buildToolbar(builder) {
          try {
            if (!builder || typeof builder.getGroup !== "function") {
              return;
            }
            const group = builder.getGroup("function");
            if (!group || typeof group.addItem !== "function") {
              return;
            }
            const items = Array.isArray(group.group?.items)
              ? group.group.items
              : [];
            if (items.some((item) => item && item.key === "inline-ai-chat")) {
              return;
            }
            group.addItem("inline-ai-chat", {
              icon: INLINE_AI_CHAT_ICON,
              active: () => false,
              onRun: () => {
                if (typeof inlineAIToolbarHandler === "function") {
                  inlineAIToolbarHandler();
                } else {
                  console.warn("[milkdown] AI Chat 功能尚未就绪");
                }
              },
            });
          } catch (error) {
            console.warn("[milkdown] 注册 AI Chat 工具失败", error);
          }
        },
      },
      [blockEditKey]: {
        buildMenu(builder) {
          try {
            registerCalloutSlashMenu(builder);
          } catch (error) {
            console.warn("[milkdown] 注册 Callout 菜单失败", error);
          }
          try {
            registerEmojiSlashMenu(builder);
          } catch (error) {
            console.warn("[milkdown] 注册 Emoji 菜单失败", error);
          }
        },
      },
      [imageBlockKey]: {
        onUpload: imageUploader,
        inlineOnUpload: imageUploader,
        blockOnUpload: imageUploader,
      },
    };

    const crepe = new Crepe({
      root: mount,
      defaultValue: initial,
      featureConfigs,
    });
    applyMilkdownPlugins(crepe.editor, toast);

    await crepe.create();
    ensureSetMarkdown(crepe);
    if (typeof crepe.setMarkdown === "function") {
      try {
        const result = crepe.setMarkdown(initial, {
          flush: true,
          silent: true,
        });
        if (result && typeof result.then === "function") {
          await result;
        }
      } catch (error) {
        console.warn("[milkdown] 初始化内容设置失败", error);
      }
    }

    if (typeof window !== "undefined") {
      const initialData =
        typeof window.__MILKDOWN_V2__ === "object"
          ? window.__MILKDOWN_V2__
          : {};
      const controller = new PostDraftController(crepe, initialData);
      controller.init();

      const inlineAI = setupInlineAI(crepe, toast, () => controller);

      crepe.editor.action((ctx) => {
        try {
          const listener = ctx.get(listenerCtx);
          if (!listener) {
            return;
          }
          listener.markdownUpdated((_ctx, markdown) => {
            if (typeof markdown !== "string") {
              return;
            }
            if (markdown === controller.currentContent) {
              return;
            }
            controller.currentContent = markdown;
            controller.postData.Content = markdown;
            controller.updateContentMetrics(markdown);
            if (markdown !== controller.lastSavedContent) {
              controller.markDirty();
            }
          });
          listener.blur(() => {
            if (typeof controller.autoSaveIfNeeded === "function") {
              void controller.autoSaveIfNeeded();
            }
          });
        } catch (error) {
          console.error("[milkdown] 监听器初始化失败", error);
        }
      });

      window.MilkdownV2 = {
        crepe,
        editor: crepe.editor,
        getMarkdown: crepe.getMarkdown,
        controller,
        createReadOnlyViewer,
        saveDraft: controller.saveDraft.bind(controller),
        publish: controller.publish.bind(controller),
        inlineAI: inlineAI
          ? {
              hideToolbar: inlineAI.hide,
              getSelection: inlineAI.readSelection,
              applyChange: (options) => inlineAI.applyChange(options),
            }
          : null,
      };

      controller.emitPostEvent("ready");
      controller.notifyPostChange("init");
    }
  } catch (error) {
    console.error("[milkdown] 初始化失败", error);
  }
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", initialize, { once: true });
} else {
  initialize();
}
