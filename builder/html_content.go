package builder

const IndexHTML = `<!DOCTYPE html>
<html lang="en" class="h-full bg-slate-950 text-slate-100">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Flow Pipeline Builder</title>
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>
    <script src="https://cdn.tailwindcss.com"></script>
    <script src="https://cdn.jsdelivr.net/npm/sortablejs@1.15.2/Sortable.min.js"></script>
    <style>
        .custom-scrollbar::-webkit-scrollbar { width: 6px; height: 6px; }
        .custom-scrollbar::-webkit-scrollbar-track { background: transparent; }
        .custom-scrollbar::-webkit-scrollbar-thumb { background: rgba(100, 116, 139, 0.4); border-radius: 3px; }
        .sortable-ghost { opacity: 0.4; background-color: #0284c7 !important; border: 2px dashed #38bdf8; }

        /* Themes */
        html[data-theme="slate"] {
            --bg-body: #020617;
        }
        html[data-theme="midnight"] {
            --bg-body: #090d16;
            filter: hue-rotate(15deg) contrast(105%);
        }
        html[data-theme="nord"] {
            --bg-body: #2e3440;
            filter: hue-rotate(190deg) saturate(90%);
        }
        html[data-theme="emerald"] {
            --bg-body: #022c22;
            filter: hue-rotate(110deg) saturate(95%);
        }
        html[data-theme="amber"] {
            --bg-body: #1c1917;
            filter: hue-rotate(345deg) saturate(110%);
        }
        html[data-theme="cyberpunk"] {
            --bg-body: #180026;
            filter: hue-rotate(270deg) saturate(140%) contrast(110%);
        }
        html[data-theme="vscode-dark-plus"] {
            --bg-body: #1e1e1e;
            filter: contrast(1.05) saturate(1.08);
        }
        html[data-theme="vscode-light-plus"] {
            --bg-body: #f3f3f3;
            filter: contrast(0.98) saturate(0.75) brightness(1.08);
        }
        html[data-theme="vscode-monokai"] {
            --bg-body: #272822;
            filter: sepia(0.18) hue-rotate(15deg) saturate(1.3) contrast(1.08);
        }
        html[data-theme="vscode-dracula"] {
            --bg-body: #282a36;
            filter: hue-rotate(245deg) saturate(1.4) brightness(0.92);
        }
        html[data-theme="visual-studio-dark"] {
            --bg-body: #1e1e1e;
            filter: contrast(1.12) saturate(1.1) brightness(0.97);
        }
        html[data-theme="visual-studio-light"] {
            --bg-body: #edf3fb;
            filter: contrast(1.02) saturate(0.8) brightness(1.03);
        }
        html[data-theme="visual-studio-blue"] {
            --bg-body: #1d3354;
            filter: none;
        }
        html[data-theme="light"] {
            filter: invert(93%) hue-rotate(180deg) contrast(95%);
        }

        html[data-theme="visual-studio-light"] body,
        html[data-theme="visual-studio-light"] .bg-slate-950,
        html[data-theme="visual-studio-light"] .bg-slate-900,
        html[data-theme="visual-studio-light"] .bg-slate-800,
        html[data-theme="visual-studio-light"] .bg-slate-700,
        html[data-theme="visual-studio-light"] .bg-slate-800\/70,
        html[data-theme="visual-studio-light"] .bg-slate-950\/60 {
            background-color: #edf3fb !important;
        }
        html[data-theme="visual-studio-light"] .border-slate-800,
        html[data-theme="visual-studio-light"] .border-slate-700,
        html[data-theme="visual-studio-light"] .border-slate-800\/80,
        html[data-theme="visual-studio-light"] .border-slate-800\/60 {
            border-color: #cfe0f3 !important;
        }
        html[data-theme="visual-studio-light"] .text-slate-200,
        html[data-theme="visual-studio-light"] .text-slate-300,
        html[data-theme="visual-studio-light"] .text-slate-400,
        html[data-theme="visual-studio-light"] .text-slate-500 {
            color: #1f2d3d !important;
        }
        html[data-theme="visual-studio-light"] .text-cyan-400,
        html[data-theme="visual-studio-light"] .text-blue-400,
        html[data-theme="visual-studio-light"] .text-blue-300 {
            color: #0d5ea7 !important;
        }

        html[data-theme="visual-studio-blue"],
        html[data-theme="visual-studio-blue"] body,
        html[data-theme="visual-studio-blue"] .bg-slate-950,
        html[data-theme="visual-studio-blue"] .bg-slate-900,
        html[data-theme="visual-studio-blue"] .bg-slate-800,
        html[data-theme="visual-studio-blue"] .bg-slate-700,
        html[data-theme="visual-studio-blue"] .bg-slate-800\/70,
        html[data-theme="visual-studio-blue"] .bg-slate-950\/60,
        html[data-theme="visual-studio-blue"] .bg-slate-900\/80,
        html[data-theme="visual-studio-blue"] .bg-slate-900\/70 {
            background-color: #1b2d48 !important;
        }
        html[data-theme="visual-studio-blue"] .bg-slate-800 {
            background-color: #2b446a !important;
        }
        html[data-theme="visual-studio-blue"] .bg-slate-700 {
            background-color: #3b5f8e !important;
        }
        html[data-theme="visual-studio-blue"] .border-slate-800,
        html[data-theme="visual-studio-blue"] .border-slate-700,
        html[data-theme="visual-studio-blue"] .border-slate-800\/80,
        html[data-theme="visual-studio-blue"] .border-slate-800\/60,
        html[data-theme="visual-studio-blue"] .border-slate-700\/60 {
            border-color: #4e729d !important;
        }
        html[data-theme="visual-studio-blue"] .text-slate-200,
        html[data-theme="visual-studio-blue"] .text-slate-300,
        html[data-theme="visual-studio-blue"] .text-slate-400,
        html[data-theme="visual-studio-blue"] .text-slate-500,
        html[data-theme="visual-studio-blue"] .text-white {
            color: #edf4ff !important;
        }
        html[data-theme="visual-studio-blue"] .text-slate-400 {
            color: #bdd3f3 !important;
        }
        html[data-theme="visual-studio-blue"] .text-cyan-400,
        html[data-theme="visual-studio-blue"] .text-blue-400,
        html[data-theme="visual-studio-blue"] .text-blue-300,
        html[data-theme="visual-studio-blue"] .text-blue-200 {
            color: #9ad0ff !important;
        }
        html[data-theme="visual-studio-blue"] .bg-blue-600,
        html[data-theme="visual-studio-blue"] .bg-blue-500,
        html[data-theme="visual-studio-blue"] .bg-blue-400 {
            background-color: #3c7dd9 !important;
        }
        html[data-theme="visual-studio-blue"] .bg-blue-600,
        html[data-theme="visual-studio-blue"] .bg-blue-500,
        html[data-theme="visual-studio-blue"] .bg-blue-400,
        html[data-theme="visual-studio-blue"] .text-blue-500 {
            color: #dfeeff !important;
        }
        html[data-theme="light"] img, html[data-theme="light"] video {
            filter: invert(100%) hue-rotate(180deg);
        }
    </style>
</head>
<body class="h-full flex flex-col font-sans select-none overflow-hidden">

    <header class="bg-slate-900 border-b border-slate-800 px-6 py-3 flex items-center justify-between shadow-md">
        <div class="flex items-center space-x-4">
            <div class="flex items-center space-x-2">
                <span class="text-2xl">🌊</span>
                <div>
                    <h1 class="text-lg font-bold tracking-tight bg-gradient-to-r from-cyan-400 to-blue-500 bg-clip-text text-transparent">FLOW BUILDER</h1>
                    <p class="text-xs text-slate-400">Visual Pipeline Designer</p>
                </div>
            </div>
            <div class="h-6 w-px bg-slate-700 mx-2"></div>
            <div class="flex items-center space-x-2" id="script-selector-container">
                {{template "script_selector" .}}
            </div>
        </div>
        <div class="flex items-center bg-slate-800 p-1 rounded-lg border border-slate-700">
            <button onclick="switchTab('pipeline')" id="tab-btn-pipeline" class="tab-btn px-4 py-1.5 text-xs font-semibold rounded-md transition-colors bg-blue-600 text-white shadow">
                📜 Pipeline (scripts.xml)
            </button>
            <button onclick="switchTab('config')" id="tab-btn-config" class="tab-btn px-4 py-1.5 text-xs font-semibold rounded-md transition-colors text-slate-300 hover:text-white">
                ⚙️ Config Overrides
            </button>
            <button onclick="switchTab('options')" id="tab-btn-options" class="tab-btn px-4 py-1.5 text-xs font-semibold rounded-md transition-colors text-slate-300 hover:text-white">
                🎛️ CLI Options
            </button>
            <button onclick="switchTab('runner')" id="tab-btn-runner" class="tab-btn px-4 py-1.5 text-xs font-semibold rounded-md transition-colors text-slate-300 hover:text-white">
                🚀 Pipeline Runner
            </button>
        </div>
        <div class="flex items-center space-x-3">
            <div class="flex items-center space-x-1.5 bg-slate-800 border border-slate-700 rounded-md px-2 py-1">
                <span class="text-xs text-slate-400">🎨</span>
                <select id="theme-selector" onchange="setTheme(this.value)" class="bg-transparent text-xs text-slate-200 font-medium focus:outline-none cursor-pointer">
                    <option value="slate" class="bg-slate-900 text-slate-200">Slate (Default)</option>
                    <option value="midnight" class="bg-slate-900 text-slate-200">Midnight Blue</option>
                    <option value="nord" class="bg-slate-900 text-slate-200">Nord Frost</option>
                    <option value="emerald" class="bg-slate-900 text-slate-200">Emerald Matrix</option>
                    <option value="amber" class="bg-slate-900 text-slate-200">Warm Amber</option>
                    <option value="cyberpunk" class="bg-slate-900 text-slate-200">Cyberpunk Neon</option>
                    <option value="vscode-dark-plus" class="bg-slate-900 text-slate-200">VS Code Dark+</option>
                    <option value="vscode-light-plus" class="bg-slate-900 text-slate-200">VS Code Light+</option>
                    <option value="vscode-monokai" class="bg-slate-900 text-slate-200">VS Code Monokai</option>
                    <option value="vscode-dracula" class="bg-slate-900 text-slate-200">VS Code Dracula</option>
                    <option value="visual-studio-dark" class="bg-slate-900 text-slate-200">Visual Studio Dark</option>
                    <option value="visual-studio-light" class="bg-slate-900 text-slate-200">Visual Studio Light</option>
                    <option value="visual-studio-blue" class="bg-slate-900 text-slate-200">Visual Studio Blue</option>
                    <option value="light" class="bg-slate-900 text-slate-200">Clean Light</option>
                </select>
            </div>
            <button onclick="confirmPurgeDatabase()" class="px-2.5 py-1.5 text-xs font-semibold bg-rose-950/40 hover:bg-rose-900/60 text-rose-300 border border-rose-800/50 rounded-md transition-all flex items-center space-x-1 shadow-sm" title="Purge SQLite database and reset to factory defaults">
                <span>🧹 Purge DB</span>
            </button>
            <button onclick="downloadActiveXML()" class="px-3 py-1.5 text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 rounded-md transition-all flex items-center space-x-1.5 shadow-sm">
                <span>⬇️ Download XML</span>
            </button>
            <button onclick="saveToFileOnDisk()" class="px-3.5 py-1.5 text-xs font-semibold bg-emerald-600 hover:bg-emerald-500 text-white rounded-md transition-all flex items-center space-x-1.5 shadow">
                <span>💾 Save to File</span>
            </button>
        </div>

    </header>
    <main class="flex-1 flex overflow-hidden">
        <div id="tab-pipeline" class="flex-1 flex w-full h-full">
            <aside class="w-80 bg-slate-900 border-r border-slate-800 flex flex-col">
                <div class="p-3 border-b border-slate-800">
                    <input type="text" id="component-filter" oninput="filterComponents(this.value)" placeholder="Search components..." 
                           class="w-full bg-slate-950 border border-slate-700 rounded-md px-3 py-1.5 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-blue-500">
                </div>
                <div class="flex-1 overflow-y-auto custom-scrollbar p-3 space-y-4" id="component-palette">
                    {{range $cat := .Categories}}
                    <div class="component-category" data-category="{{$cat}}">
                        <h3 class="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2 px-1">{{$cat}}</h3>
                        <div class="space-y-1.5">
                            {{range $.Components}}
                            {{if eq .Category $cat}}
                            <div class="component-card group bg-slate-950 hover:bg-slate-800 border border-slate-800 hover:border-slate-700 p-2.5 rounded-lg transition-all cursor-pointer shadow-sm"
                                 onclick="openAddNodeModal('{{.Type}}', '{{.Name}}', '{{.Section}}')"
                                 data-name="{{.Name}}" data-type="{{.Type}}" data-desc="{{.Description}}">
                                <div class="flex items-center justify-between mb-1">
                                    <span class="text-xs font-semibold text-slate-200 group-hover:text-cyan-400 transition-colors">&lt;{{.Tag}}&gt; {{.Name}}</span>
                                    <span class="text-[10px] uppercase font-mono px-1.5 py-0.5 rounded bg-slate-800 text-slate-400 border border-slate-700">{{.Section}}</span>
                                </div>
                                <p class="text-[11px] text-slate-400 line-clamp-2 leading-relaxed">{{.Description}}</p>
                            </div>
                            {{end}}
                            {{end}}
                        </div>
                    </div>
                    {{end}}
                </div>
            </aside>
            <section class="flex-1 flex flex-col bg-slate-950 overflow-hidden">
                <div class="bg-slate-900 border-b border-slate-800 px-6 py-2.5 flex items-center justify-between">
                    <div class="flex items-center space-x-2 text-xs" id="pipeline-sections-nav">
                        <span class="text-slate-400 font-medium mr-1">Pipeline Sections:</span>
                        <a href="#section-variables" id="nav-section-variables" onclick="navigateToSection('variables', event)" class="section-nav-link px-2.5 py-1 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded border border-slate-700 transition">Variables</a>
                        <a href="#section-databases" id="nav-section-databases" onclick="navigateToSection('databases', event)" class="section-nav-link px-2.5 py-1 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded border border-slate-700 transition">Databases</a>
                        <a href="#section-preflight" id="nav-section-preflight" onclick="navigateToSection('preflight', event)" class="section-nav-link px-2.5 py-1 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded border border-slate-700 transition">Preflight</a>
                        <a href="#section-flow" id="nav-section-flow" onclick="navigateToSection('flow', event)" class="section-nav-link px-2.5 py-1 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded border border-slate-700 transition">Main Flow</a>
                    </div>
                    <div class="text-xs text-slate-400">Drag cards or use ▲/▼ to re-order sequence</div>
                </div>
                <div class="flex-1 overflow-y-auto custom-scrollbar p-6 space-y-6 scroll-smooth" id="canvas-content"
                     hx-get="/api/canvas?script_id={{.ActiveScript.ID}}" hx-trigger="refreshCanvas from:body" hx-swap="innerHTML">
                    {{template "canvas_nodes" .}}
                </div>
            </section>
            <aside class="w-96 bg-slate-900 border-l border-slate-800 flex flex-col">
                <div class="p-3 border-b border-slate-800 flex items-center justify-between">
                    <span class="text-xs font-bold text-slate-300 uppercase tracking-wider">Live XML Preview</span>
                    <button onclick="copyXMLPreview()" class="text-xs text-cyan-400 hover:text-cyan-300 font-medium">📋 Copy</button>
                </div>
                <div class="flex-1 overflow-auto custom-scrollbar p-3 bg-slate-950">
                    <pre id="live-xml-preview" class="text-[11px] font-mono text-cyan-300/90 whitespace-pre leading-relaxed"></pre>
                </div>
            </aside>
        </div>
        <div id="tab-config" class="hidden flex-1 flex flex-col bg-slate-950 p-6 overflow-hidden">
            <div class="max-w-5xl w-full mx-auto flex-1 flex flex-col bg-slate-900 border border-slate-800 rounded-xl overflow-hidden shadow-2xl">
                <div class="px-6 py-4 border-b border-slate-800 flex items-center justify-between bg-slate-900/70">
                    <div>
                        <h2 class="text-base font-bold text-white">Configuration Overrides (CONFIG.xml)</h2>
                        <p class="text-xs text-slate-400">Environment variable & DB overrides loaded via the -config flag.</p>
                    </div>
                    <button onclick="saveConfigFile()" class="px-4 py-1.5 text-xs font-semibold bg-blue-600 hover:bg-blue-500 text-white rounded-md transition shadow">
                        Save Configuration
                    </button>
                </div>
                <div class="flex-1 p-4 bg-slate-950">
                    <textarea id="config-editor" class="w-full h-full bg-slate-950 text-cyan-300 font-mono text-xs p-4 rounded border border-slate-800 focus:outline-none focus:border-blue-500 resize-none leading-relaxed custom-scrollbar">{{.DefaultConfigContent}}</textarea>
                </div>
            </div>
        </div>
        <div id="tab-options" class="hidden flex-1 flex flex-col bg-slate-950 p-6 overflow-hidden">
            <div class="max-w-5xl w-full mx-auto flex-1 flex flex-col bg-slate-900 border border-slate-800 rounded-xl overflow-hidden shadow-2xl">
                <div class="px-6 py-4 border-b border-slate-800 flex items-center justify-between bg-slate-900/70">
                    <div>
                        <h2 class="text-base font-bold text-white">CLI Options File (options.xml)</h2>
                        <p class="text-xs text-slate-400">CLI parameter defaults loaded via the -options flag.</p>
                    </div>
                    <button onclick="saveOptionsFile()" class="px-4 py-1.5 text-xs font-semibold bg-blue-600 hover:bg-blue-500 text-white rounded-md transition shadow">
                        Save Options
                    </button>
                </div>
                <div class="flex-1 p-4 bg-slate-950">
                    <textarea id="options-editor" class="w-full h-full bg-slate-950 text-cyan-300 font-mono text-xs p-4 rounded border border-slate-800 focus:outline-none focus:border-blue-500 resize-none leading-relaxed custom-scrollbar">{{.DefaultOptionsContent}}</textarea>
                </div>
            </div>
        </div>
        <div id="tab-runner" class="hidden flex-1 flex flex-col bg-slate-950 p-6 overflow-hidden">
            <div class="max-w-6xl w-full mx-auto flex-1 flex flex-col bg-slate-900 border border-slate-800 rounded-xl overflow-hidden shadow-2xl">
                <div class="px-6 py-4 border-b border-slate-800 bg-slate-900/70 flex flex-col gap-3">
                    <div class="flex flex-wrap items-center justify-between gap-3">
                        <div>
                            <h2 class="text-base font-bold text-white flex items-center space-x-2">
                                <span>🚀 Execute Pipeline</span>
                                <span class="text-[11px] font-normal px-2 py-0.5 rounded bg-blue-500/20 text-blue-300 border border-blue-500/30">Live Stream</span>
                            </h2>
                            <p class="text-xs text-slate-400">Select pipeline script, config, and options from the filesystem or execute active draft.</p>
                        </div>
                        <div class="flex items-center space-x-3">
                            <div class="bg-slate-950 border border-slate-800 rounded-lg p-0.5 flex items-center space-x-1">
                                <button type="button" onclick="setRunnerSource('file')" id="source-btn-file" class="px-3 py-1 text-xs font-semibold rounded-md transition-colors bg-blue-600 text-white shadow">
                                    📂 Filesystem Script
                                </button>
                                <button type="button" onclick="setRunnerSource('builder')" id="source-btn-builder" class="px-3 py-1 text-xs font-semibold rounded-md transition-colors text-slate-400 hover:text-white">
                                    🎨 Builder Draft
                                </button>
                            </div>
                            <button onclick="startPipelineExecution()" id="execute-btn" class="px-5 py-2 text-xs font-bold bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg transition shadow flex items-center space-x-1.5 cursor-pointer">
                                <span>▶ Run Execution</span>
                            </button>
                        </div>
                    </div>
                    <div class="grid grid-cols-1 md:grid-cols-3 gap-3 pt-1">
                        <div class="flex flex-col space-y-1">
                            <div class="flex items-center justify-between">
                                <label class="text-[11px] font-semibold text-slate-300">📜 Pipeline Script (<span class="text-amber-400">*</span>)</label>
                                <span id="runner-source-label" class="text-[10px] text-cyan-400 font-mono">source: filesystem</span>
                            </div>
                            <div class="flex items-center">
                                <input type="text" id="runner-script-file" list="quick-xml-files" value="scripts.xml" placeholder="scripts.xml" class="bg-slate-950 border border-slate-700 rounded-l px-2.5 py-1.5 text-xs text-slate-200 w-full focus:outline-none focus:border-blue-500 font-mono">
                                <button type="button" onclick="openFileBrowser('runner-script-file', '.xml')" title="Browse filesystem for script XML" class="bg-slate-800 hover:bg-slate-700 text-slate-200 border-y border-r border-slate-700 rounded-r px-2.5 py-1.5 text-xs font-medium transition flex items-center space-x-1 whitespace-nowrap cursor-pointer">
                                    <span>📂 Browse</span>
                                </button>
                            </div>
                        </div>
                        <div class="flex flex-col space-y-1">
                            <div class="flex items-center justify-between">
                                <label class="text-[11px] font-semibold text-slate-300">⚙️ Config Overrides (optional)</label>
                                <span class="text-[10px] text-slate-500 font-mono">-config</span>
                            </div>
                            <div class="flex items-center">
                                <input type="text" id="runner-config-file" list="quick-xml-files" value="" placeholder="CONFIG.xml (optional)" class="bg-slate-950 border border-slate-700 rounded-l px-2.5 py-1.5 text-xs text-slate-200 w-full focus:outline-none focus:border-blue-500 font-mono">
                                <button type="button" onclick="document.getElementById('runner-config-file').value=''" title="Clear config file" class="bg-slate-900 hover:bg-slate-800 text-slate-400 hover:text-slate-200 border-y border-slate-700 px-2 py-1.5 text-xs transition cursor-pointer">✕</button>
                                <button type="button" onclick="openFileBrowser('runner-config-file', '.xml')" title="Browse filesystem for CONFIG.xml" class="bg-slate-800 hover:bg-slate-700 text-slate-200 border-y border-r border-slate-700 rounded-r px-2.5 py-1.5 text-xs font-medium transition flex items-center space-x-1 whitespace-nowrap cursor-pointer">
                                    <span>📂 Browse</span>
                                </button>
                            </div>
                        </div>
                        <div class="flex flex-col space-y-1">
                            <div class="flex items-center justify-between">
                                <label class="text-[11px] font-semibold text-slate-300">🎛️ CLI Options (optional)</label>
                                <span class="text-[10px] text-slate-500 font-mono">-options</span>
                            </div>
                            <div class="flex items-center">
                                <input type="text" id="runner-options-file" list="quick-xml-files" value="" placeholder="options.xml (optional)" class="bg-slate-950 border border-slate-700 rounded-l px-2.5 py-1.5 text-xs text-slate-200 w-full focus:outline-none focus:border-blue-500 font-mono">
                                <button type="button" onclick="document.getElementById('runner-options-file').value=''" title="Clear options file" class="bg-slate-900 hover:bg-slate-800 text-slate-400 hover:text-slate-200 border-y border-slate-700 px-2 py-1.5 text-xs transition cursor-pointer">✕</button>
                                <button type="button" onclick="openFileBrowser('runner-options-file', '.xml')" title="Browse filesystem for options.xml" class="bg-slate-800 hover:bg-slate-700 text-slate-200 border-y border-r border-slate-700 rounded-r px-2.5 py-1.5 text-xs font-medium transition flex items-center space-x-1 whitespace-nowrap cursor-pointer">
                                    <span>📂 Browse</span>
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
                <datalist id="quick-xml-files"></datalist>
                <div class="flex-1 flex flex-col overflow-hidden p-4 bg-slate-950 space-y-4">
                    <div id="execution-status-bar" class="px-4 py-2.5 bg-slate-900 border border-slate-800 rounded-lg flex items-center justify-between text-xs">
                        <div class="flex items-center space-x-2">
                            <span id="status-indicator" class="h-2.5 w-2.5 rounded-full bg-slate-500"></span>
                            <span id="status-text" class="font-medium text-slate-300">Ready to execute</span>
                        </div>
                        <div id="status-timer" class="text-slate-400 font-mono text-xs">00:00.000</div>
                    </div>
                    <div class="h-48 overflow-y-auto custom-scrollbar border border-slate-800 rounded-lg bg-slate-900/40">
                        <table class="w-full text-left text-xs text-slate-300">
                            <thead class="bg-slate-900 text-slate-400 border-b border-slate-800 sticky top-0 uppercase text-[10px]">
                                <tr>
                                    <th class="p-2.5">Step / Node ID</th>
                                    <th class="p-2.5">Kind</th>
                                    <th class="p-2.5">Status</th>
                                    <th class="p-2.5">Return Code</th>
                                    <th class="p-2.5">Duration</th>
                                    <th class="p-2.5">Output Summary</th>
                                </tr>
                            </thead>
                            <tbody id="execution-results-body" class="divide-y divide-slate-800/60 font-mono text-[11px]">
                                <tr>
                                    <td colspan="6" class="p-4 text-center text-slate-500">No execution active. Click "Run Execution" to begin.</td>
                                </tr>
                            </tbody>
                        </table>
                    </div>
                    <div class="flex-1 flex flex-col bg-slate-900 border border-slate-800 rounded-lg overflow-hidden">
                        <div class="px-3 py-1.5 bg-slate-900 border-b border-slate-800 text-[11px] font-mono text-slate-400 flex items-center justify-between">
                            <span>Console Stream Logs</span>
                            <button onclick="document.getElementById('terminal-log').textContent=''" class="hover:text-slate-200">Clear</button>
                        </div>
                        <pre id="terminal-log" class="flex-1 p-3 overflow-y-auto custom-scrollbar text-[11px] font-mono text-emerald-400 whitespace-pre-wrap leading-relaxed select-text bg-black/50"></pre>
                    </div>
                </div>
            </div>
        </div>
    </main>
    <div id="node-modal" class="hidden fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 z-50">
        <div class="bg-slate-900 border border-slate-800 rounded-xl max-w-xl w-full p-6 shadow-2xl flex flex-col space-y-4 max-h-[90vh]">
            <div class="flex items-center justify-between border-b border-slate-800 pb-3">
                <div>
                    <h3 id="modal-title" class="text-sm font-bold text-white">Configure Component</h3>
                    <p id="modal-subtitle" class="text-xs text-slate-400"></p>
                </div>
                <button onclick="closeNodeModal()" class="text-slate-400 hover:text-white">&times;</button>
            </div>
            <form id="node-form" onsubmit="submitNodeModal(event)" class="flex-1 overflow-y-auto custom-scrollbar space-y-3.5 pr-1">
                <input type="hidden" id="modal-node-id" name="node_id">
                <input type="hidden" id="modal-node-type" name="node_type">
                <div class="space-y-1">
                    <label class="text-xs font-medium text-slate-300">Target Section</label>
                    <select id="modal-node-section" name="section" class="w-full bg-slate-950 border border-slate-700 rounded p-1.5 text-xs text-white focus:outline-none focus:border-blue-500">
                        <option value="flow">4. Pipeline Flow (&lt;flow&gt; sequential steps)</option>
                        <option value="preflight">3. Preflight Checks (&lt;preflight&gt;)</option>
                        <option value="variables">1. Pipeline Variables (&lt;variables&gt;)</option>
                        <option value="databases">2. Database Connections (&lt;databases&gt;)</option>
                    </select>
                </div>
                <div id="modal-fields-container" class="space-y-3"></div>
                <div class="pt-2 border-t border-slate-800">
                    <div class="flex items-center justify-between mb-2">
                        <label class="text-xs font-semibold text-slate-400 uppercase tracking-wider">Custom / Extra Attributes</label>
                        <button type="button" onclick="addCustomAttributeRow('', '', getAllowedCustomAttributeNames(window.currentNodeMeta || getCurrentNodeMeta()))" class="text-[11px] text-cyan-400 hover:text-cyan-300 font-medium">＋ Add Attribute</button>
                    </div>
                    <div id="modal-custom-attrs" class="space-y-2"></div>
                </div>
                <div id="modal-content-container" class="hidden space-y-1.5">
                    <label id="modal-content-label" class="block text-xs font-medium text-slate-300">Content / Script</label>
                    <textarea id="modal-content-text" name="content" rows="6" class="w-full bg-slate-950 border border-slate-700 rounded-md p-2.5 text-xs text-cyan-300 font-mono focus:outline-none focus:border-blue-500 custom-scrollbar"></textarea>
                </div>
                <div class="flex items-center justify-end space-x-2.5 pt-4 border-t border-slate-800">
                    <button type="button" onclick="closeNodeModal()" class="px-3.5 py-1.5 text-xs font-semibold bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-md transition">Cancel</button>
                    <button type="submit" class="px-4 py-1.5 text-xs font-semibold bg-blue-600 hover:bg-blue-500 text-white rounded-md transition shadow">Save Component</button>
                </div>
            </form>
        </div>
    </div>
    <div id="new-script-modal" class="hidden fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 z-50">
        <div class="bg-slate-900 border border-slate-800 rounded-xl max-w-md w-full p-6 shadow-2xl flex flex-col space-y-4">
            <h3 class="text-sm font-bold text-white">Create New Pipeline</h3>
            <form onsubmit="submitNewScript(event)" class="space-y-3">
                <div>
                    <label class="block text-xs text-slate-300 mb-1">Pipeline Name</label>
                    <input type="text" id="new-script-name" required class="w-full bg-slate-950 border border-slate-700 rounded p-2 text-xs text-white focus:outline-none focus:border-blue-500">
                </div>
                <div>
                    <label class="block text-xs text-slate-300 mb-1">Description</label>
                    <input type="text" id="new-script-desc" class="w-full bg-slate-950 border border-slate-700 rounded p-2 text-xs text-white focus:outline-none focus:border-blue-500">
                </div>
                <div class="flex justify-end space-x-2 pt-2">
                    <button type="button" onclick="document.getElementById('new-script-modal').classList.add('hidden')" class="px-3 py-1.5 text-xs bg-slate-800 text-slate-300 rounded">Cancel</button>
                    <button type="submit" class="px-4 py-1.5 text-xs bg-blue-600 text-white rounded">Create</button>
                </div>
            </form>
        </div>
    </div>
    <div id="copy-pipeline-modal" class="hidden fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 z-50">
        <div class="bg-slate-900 border border-slate-800 rounded-xl max-w-md w-full p-6 shadow-2xl flex flex-col space-y-4">
            <div class="flex items-center justify-between border-b border-slate-800 pb-3">
                <div class="flex items-center space-x-2">
                    <span class="text-base">📋</span>
                    <h3 class="text-sm font-bold text-white">Duplicate Pipeline</h3>
                </div>
                <button onclick="document.getElementById('copy-pipeline-modal').classList.add('hidden')" class="text-slate-400 hover:text-white">&times;</button>
            </div>
            <form onsubmit="submitCopyScript(event)" class="space-y-3">
                <div>
                    <label class="block text-xs text-slate-300 mb-1">New Pipeline Name</label>
                    <input type="text" id="copy-script-name" required class="w-full bg-slate-950 border border-slate-700 rounded p-2 text-xs text-white focus:outline-none focus:border-blue-500">
                </div>
                <p class="text-[11px] text-slate-400">All variables, databases, preflight gates, and flow steps will be copied into the new pipeline draft.</p>
                <div class="flex justify-end space-x-2 pt-2">
                    <button type="button" onclick="document.getElementById('copy-pipeline-modal').classList.add('hidden')" class="px-3 py-1.5 text-xs bg-slate-800 text-slate-300 rounded">Cancel</button>
                    <button type="submit" class="px-4 py-1.5 text-xs bg-blue-600 hover:bg-blue-500 text-white rounded font-semibold shadow">Create Copy</button>
                </div>
            </form>
        </div>
    </div>
    <div id="import-pipeline-modal" class="hidden fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 z-50">
        <div class="bg-slate-900 border border-slate-800 rounded-xl max-w-lg w-full p-6 shadow-2xl flex flex-col space-y-4">
            <div class="flex items-center justify-between border-b border-slate-800 pb-3">
                <div class="flex items-center space-x-2">
                    <span class="text-base">📂</span>
                    <h3 class="text-sm font-bold text-white">Import Pipeline from Filesystem</h3>
                </div>
                <button onclick="document.getElementById('import-pipeline-modal').classList.add('hidden')" class="text-slate-400 hover:text-white">&times;</button>
            </div>
            <form onsubmit="submitImportScript(event)" class="space-y-3">
                <div>
                    <label class="block text-xs text-slate-300 mb-1">Pipeline File (.xml)</label>
                    <div class="flex space-x-2">
                        <input type="text" id="import-file-path" list="quick-xml-files" placeholder="examples/check_two_tables_in_parallel_take_action.xml" required class="flex-1 bg-slate-950 border border-slate-700 rounded p-2 text-xs text-white focus:outline-none focus:border-blue-500 font-mono">
                        <button type="button" onclick="browseForImportFile()" class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 border border-slate-700 text-slate-200 text-xs rounded transition flex items-center space-x-1">
                            <span>Browse...</span>
                        </button>
                    </div>
                </div>
                <div>
                    <label class="block text-xs text-slate-300 mb-1">Custom Pipeline Name (Optional)</label>
                    <input type="text" id="import-script-name" placeholder="Leave empty to use XML/file name" class="w-full bg-slate-950 border border-slate-700 rounded p-2 text-xs text-white focus:outline-none focus:border-blue-500">
                </div>
                <p class="text-[11px] text-slate-400">The XML file will be parsed and imported into SQLite as an editable visual draft with all variables, databases, and flow nodes.</p>
                <div class="flex justify-end space-x-2 pt-2">
                    <button type="button" onclick="document.getElementById('import-pipeline-modal').classList.add('hidden')" class="px-3 py-1.5 text-xs bg-slate-800 text-slate-300 rounded">Cancel</button>
                    <button type="submit" class="px-4 py-1.5 text-xs bg-cyan-600 hover:bg-cyan-500 text-white rounded font-semibold shadow">Import Pipeline</button>
                </div>
            </form>
        </div>
    </div>
    <div id="delete-pipeline-modal" class="hidden fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 z-50">
        <div class="bg-slate-900 border border-rose-900/40 rounded-xl max-w-md w-full p-6 shadow-2xl flex flex-col space-y-4">
            <div class="flex items-center space-x-2 text-rose-400 border-b border-slate-800 pb-3">
                <span class="text-xl">⚠️</span>
                <h3 class="text-sm font-bold text-white">Delete Pipeline</h3>
            </div>
            <p class="text-xs text-slate-300">Are you sure you want to delete pipeline <span id="delete-pipeline-name" class="font-bold text-rose-400"></span>?</p>
            <p class="text-[11px] text-slate-400">All canvas nodes and configuration for this pipeline will be permanently removed from SQLite.</p>
            <div class="flex justify-end space-x-2 pt-2 border-t border-slate-800">
                <button type="button" onclick="document.getElementById('delete-pipeline-modal').classList.add('hidden')" class="px-3 py-1.5 text-xs bg-slate-800 text-slate-300 rounded">Cancel</button>
                <button type="button" onclick="submitDeleteScript()" class="px-4 py-1.5 text-xs bg-rose-600 hover:bg-rose-500 text-white font-semibold rounded shadow">Delete Pipeline</button>
            </div>
        </div>
    </div>
    <div id="purge-db-modal" class="hidden fixed inset-0 bg-black/75 backdrop-blur-sm flex items-center justify-center p-4 z-50">
        <div class="bg-slate-900 border border-red-800 rounded-xl max-w-md w-full p-6 shadow-2xl flex flex-col space-y-4">
            <div class="flex items-center space-x-2 text-red-400 border-b border-slate-800 pb-3">
                <span class="text-xl">🧹</span>
                <h3 class="text-sm font-bold text-white">Purge SQLite Database</h3>
            </div>
            <div class="p-3 bg-red-950/30 border border-red-900/50 rounded-lg text-xs text-red-300 space-y-1">
                <p class="font-bold">⚠️ DANGER: Permanent Reset</p>
                <p>This will permanently erase ALL pipelines, canvas steps, config overrides, and CLI options drafts in the SQLite database.</p>
                <p>The database will be re-initialized with fresh factory default drafts.</p>
            </div>
            <p class="text-xs text-slate-300">Are you absolutely sure you want to proceed?</p>
            <div class="flex justify-end space-x-2 pt-2 border-t border-slate-800">
                <button type="button" onclick="document.getElementById('purge-db-modal').classList.add('hidden')" class="px-3 py-1.5 text-xs bg-slate-800 text-slate-300 rounded">Cancel</button>
                <button type="button" onclick="submitPurgeDatabase()" class="px-4 py-1.5 text-xs bg-red-600 hover:bg-red-500 text-white font-semibold rounded shadow">Yes, Purge Database</button>
            </div>
        </div>
    </div>
    <div id="picker-modal" class="hidden fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 z-50">
        <div class="bg-slate-900 border border-slate-800 rounded-xl max-w-2xl w-full p-6 shadow-2xl flex flex-col space-y-4 max-h-[85vh]">
            <div class="flex items-center justify-between border-b border-slate-800 pb-3">
                <div>
                    <h3 id="picker-modal-title" class="text-sm font-bold text-white">Select Component to Add</h3>
                    <p id="picker-modal-subtitle" class="text-xs text-slate-400">Choose a step type for this section.</p>
                </div>
                <button onclick="closeComponentPicker()" class="text-slate-400 hover:text-white">&times;</button>
            </div>
            <div class="p-1">
                <input type="text" id="picker-filter-input" oninput="filterPickerComponents(this.value)" placeholder="Filter components (e.g. assert, sql, http, template)..."
                       class="w-full bg-slate-950 border border-slate-700 rounded-md px-3 py-2 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-blue-500">
            </div>
            <div class="flex-1 overflow-y-auto custom-scrollbar space-y-2 pr-1" id="picker-components-list"></div>
            <div class="flex items-center justify-end pt-3 border-t border-slate-800">
                <button type="button" onclick="closeComponentPicker()" class="px-3.5 py-1.5 text-xs font-semibold bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-md transition">Close</button>
            </div>
        </div>
    </div>
    <div id="file-browser-modal" class="hidden fixed inset-0 bg-black/75 backdrop-blur-sm flex items-center justify-center p-4 z-50">
        <div class="bg-slate-900 border border-slate-800 rounded-xl max-w-2xl w-full p-5 shadow-2xl flex flex-col space-y-3 max-h-[85vh]">
            <div class="flex items-center justify-between border-b border-slate-800 pb-2.5">
                <div class="flex items-center space-x-2">
                    <span class="text-base">📂</span>
                    <div>
                        <h3 id="file-browser-title" class="text-sm font-bold text-white">Select File from Filesystem</h3>
                        <p id="file-browser-subtitle" class="text-xs text-slate-400">Choose a file from disk</p>
                    </div>
                </div>
                <button onclick="closeFileBrowser()" class="text-slate-400 hover:text-white text-lg leading-none cursor-pointer">&times;</button>
            </div>
            
            <div class="flex items-center space-x-2 bg-slate-950 border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-slate-300">
                <span class="text-slate-500 font-mono">📁</span>
                <span id="file-browser-current-path" class="font-mono text-cyan-400 truncate flex-1">.</span>
                <button onclick="browseUp()" id="file-browser-up-btn" title="Go up to parent directory" class="px-2 py-0.5 text-[11px] font-semibold bg-slate-800 hover:bg-slate-700 text-slate-200 rounded border border-slate-700 transition cursor-pointer">
                    ⬆ Up
                </button>
                <button onclick="refreshFileBrowser()" title="Refresh directory" class="px-2 py-0.5 text-[11px] font-semibold bg-slate-800 hover:bg-slate-700 text-slate-200 rounded border border-slate-700 transition cursor-pointer">
                    🔄
                </button>
            </div>

            <div>
                <input type="text" id="file-browser-filter" oninput="filterBrowseResults(this.value)" placeholder="Filter files in this directory..."
                       class="w-full bg-slate-950 border border-slate-700 rounded-md px-3 py-1.5 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-blue-500">
            </div>

            <div class="flex-1 overflow-y-auto custom-scrollbar border border-slate-800/80 rounded-lg bg-slate-950/60 p-1 min-h-[260px] max-h-[360px]" id="file-browser-list">
            </div>

            <div class="flex items-center justify-between pt-2 border-t border-slate-800 text-xs">
                <div class="text-slate-400 text-[11px]" id="file-browser-count-text">0 items</div>
                <button type="button" onclick="closeFileBrowser()" class="px-3.5 py-1.5 font-semibold bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-md transition cursor-pointer">Cancel</button>
            </div>
        </div>
    </div>
    <script>
        let rawCatalog = {{.CatalogJSON}};
        const catalogData = Array.isArray(rawCatalog) ? rawCatalog : (typeof rawCatalog === 'string' ? JSON.parse(rawCatalog || '[]') : []);
        let currentScriptId = {{.ActiveScript.ID}};

        function switchTab(tab) {
            ['pipeline', 'config', 'options', 'runner'].forEach(t => {
                const el = document.getElementById('tab-' + t);
                const btn = document.getElementById('tab-btn-' + t);
                if (t === tab) {
                    el.classList.remove('hidden');
                    btn.classList.add('bg-blue-600', 'text-white', 'shadow');
                    btn.classList.remove('text-slate-300');
                } else {
                    el.classList.add('hidden');
                    btn.classList.remove('bg-blue-600', 'text-white', 'shadow');
                    btn.classList.add('text-slate-300');
                }
            });
            if (tab === 'pipeline') {
                updatePreview();
                setTimeout(updateActiveSectionFromScroll, 50);
            }
            if (tab === 'runner') populateQuickFilesList();
        }

        function filterComponents(q) {
            q = q.toLowerCase();
            document.querySelectorAll('.component-card').forEach(c => {
                const match = c.dataset.name.toLowerCase().includes(q) || c.dataset.type.toLowerCase().includes(q) || c.dataset.desc.toLowerCase().includes(q);
                c.style.display = match ? 'block' : 'none';
            });
        }

        function initSortables() {
            ['variables', 'databases', 'preflight', 'flow'].forEach(sec => {
                const container = document.getElementById('nodes-' + sec);
                if (container && !container.sortableInstance) {
                    container.sortableInstance = new Sortable(container, {
                        group: sec,
                        animation: 150,
                        ghostClass: 'sortable-ghost',
                        handle: '.drag-handle',
                        onEnd: function() {
                            const ids = Array.from(container.children).map(c => parseInt(c.dataset.nodeId));
                            fetch('/api/nodes/reorder', {
                                method: 'POST',
                                headers: {'Content-Type': 'application/json'},
                                body: JSON.stringify({script_id: currentScriptId, section: sec, node_ids: ids})
                            }).then(() => updatePreview());
                        }
                    });
                }
            });
        }

        function updatePreview() {
            fetch('/api/preview?script_id=' + currentScriptId)
                .then(r => r.text())
                .then(xml => {
                    const el = document.getElementById('live-xml-preview');
                    if (el) el.textContent = xml;
                });
        }

        let activePickerSection = 'flow';

        function openComponentPicker(targetSection) {
            activePickerSection = targetSection || 'flow';
            const modal = document.getElementById('picker-modal');
            const title = document.getElementById('picker-modal-title');
            const subtitle = document.getElementById('picker-modal-subtitle');
            const filterInput = document.getElementById('picker-filter-input');
            filterInput.value = '';

            const sectionLabels = {
                'preflight': 'Preflight Step (<preflight>)',
                'flow': 'Flow Step (<flow>)',
                'variables': 'Pipeline Variable (<variables>)',
                'databases': 'Database Connection (<databases>)'
            };
            title.textContent = 'Add Component to ' + (sectionLabels[activePickerSection] || activePickerSection);
            subtitle.textContent = activePickerSection === 'preflight'
                ? 'Select any validation gate, assertion, SQL check, HTTP probe, or script for preflight.'
                : 'Select any task, conditional, loop, or step for the pipeline flow.';

            renderPickerList('');
            modal.classList.remove('hidden');
            setTimeout(() => filterInput.focus(), 50);
        }

        function closeComponentPicker() {
            const modal = document.getElementById('picker-modal');
            if (modal) modal.classList.add('hidden');
        }

        function filterPickerComponents(query) {
            renderPickerList(query.toLowerCase());
        }

        function renderPickerList(query) {
            const container = document.getElementById('picker-components-list');
            container.innerHTML = '';

            // Filter components that are suitable for activePickerSection
            const items = catalogData.filter(c => {
                if (c.type === 'preflight') return false; // Container itself is already in the canvas
                if (activePickerSection === 'variables') return c.type === 'variable';
                if (activePickerSection === 'databases') return c.type === 'database';
                if (activePickerSection === 'preflight' || activePickerSection === 'flow') {
                    // Preflight and flow can execute tasks, assertions, queries, templates, conditionals, scripts, etc.
                    if (c.type === 'variable' || c.type === 'database') return false;
                }
                if (!query) return true;
                return c.name.toLowerCase().includes(query) ||
                       c.tag.toLowerCase().includes(query) ||
                       c.type.toLowerCase().includes(query) ||
                       c.description.toLowerCase().includes(query) ||
                       (c.category && c.category.toLowerCase().includes(query));
            });

            if (items.length === 0) {
                container.innerHTML = '<div class="p-6 text-center text-xs text-slate-500">No components match your filter.</div>';
                return;
            }

            items.forEach(c => {
                const div = document.createElement('div');
                div.className = 'group bg-slate-950 hover:bg-slate-800 border border-slate-800 hover:border-slate-700 p-3 rounded-lg transition-all cursor-pointer flex items-center justify-between';
                div.onclick = () => {
                    closeComponentPicker();
                    openAddNodeModal(c.type, c.name, activePickerSection);
                };
                div.innerHTML = '<div>' +
                    '<div class="flex items-center space-x-2">' +
                        '<span class="text-xs font-bold text-slate-200 group-hover:text-cyan-400 transition-colors">&lt;' + c.tag + '&gt; ' + c.name + '</span>' +
                        '<span class="text-[10px] uppercase font-mono px-1.5 py-0.5 rounded bg-slate-800 text-slate-400 border border-slate-700">' + (c.category || 'Step') + '</span>' +
                    '</div>' +
                    '<p class="text-[11px] text-slate-400 mt-0.5 leading-relaxed">' + c.description + '</p>' +
                '</div>' +
                '<span class="text-xs text-slate-500 group-hover:text-white transition">Select &rarr;</span>';
                container.appendChild(div);
            });
        }

        function openAddNodeModal(nodeType, name, defaultSection) {
            const meta = catalogData.find(c => c.type === nodeType);
            if (!meta) return;
            const targetSec = defaultSection || meta.section || 'flow';
            document.getElementById('modal-node-id').value = '';
            document.getElementById('modal-node-type').value = nodeType;
            const secSelect = document.getElementById('modal-node-section');
            if (secSelect) secSelect.value = targetSec;
            document.getElementById('modal-title').textContent = 'Add <' + meta.tag + '> ' + meta.name;
            document.getElementById('modal-subtitle').textContent = meta.description;
            document.getElementById('modal-content-text').value = '';
            renderModalFields(meta, {});
            document.getElementById('node-modal').classList.remove('hidden');
        }

        function openEditNode(nodeId) {
            fetch('/api/nodes/get?id=' + nodeId)
                .then(r => {
                    if (!r.ok) throw new Error('HTTP ' + r.status);
                    return r.json();
                })
                .then(node => {
                    const meta = catalogData.find(c => c.type === node.node_type) || {
                        type: node.node_type,
                        tag: node.node_type,
                        name: node.node_type,
                        description: 'Custom component',
                        fields: [],
                        has_content: !!node.content_text
                    };
                    document.getElementById('modal-node-id').value = node.id;
                    document.getElementById('modal-node-type').value = node.node_type;
                    document.getElementById('modal-node-section').value = node.section;
                    document.getElementById('modal-title').textContent = 'Edit <' + meta.tag + '> ' + (meta.name || node.node_type);
                    document.getElementById('modal-subtitle').textContent = meta.description || '';
                    renderModalFields(meta, node.attributes || {});
                    const contentBox = document.getElementById('modal-content-container');
                    if (meta.has_content || (node.content_text && node.content_text.trim() !== '')) {
                        contentBox.classList.remove('hidden');
                        document.getElementById('modal-content-text').value = node.content_text || '';
                    } else {
                        contentBox.classList.add('hidden');
                    }
                    document.getElementById('node-modal').classList.remove('hidden');
                })
                .catch(err => {
                    console.error('Error fetching node:', err);
                    alert('Failed to load node: ' + err);
                });
        }

        function openEditNodeModal(nodeId, nodeType, section, attrs, content) {
            openEditNode(nodeId);
        }

        function getCurrentNodeMeta() {
            const nodeType = document.getElementById('modal-node-type')?.value;
            return catalogData.find(c => c.type === nodeType) || { type: nodeType, fields: [] };
        }

        function getAllowedCustomAttributeNames(meta) {
            if (!meta || !Array.isArray(meta.fields)) return [];
            return (meta.fields || []).map(f => f.name).filter(Boolean).sort();
        }

        function addCustomAttributeRow(name, value, allowedNames) {
            const container = document.getElementById('modal-custom-attrs');
            if (!container) return;

            const row = document.createElement('div');
            row.className = 'flex items-center space-x-2 custom-attr-row';
            const safeVal = (value || '').replace(/"/g, '&quot;');
            const attrList = Array.isArray(allowedNames) && allowedNames.length > 0 ? allowedNames : [];
            const currentName = (name || '').trim();
            const currentIsAllowed = attrList.includes(currentName);

            let keyHtml = '';
            if (attrList.length > 0) {
                const opts = attrList.map(attr => {
                    const selected = attr === currentName ? ' selected' : '';
                    return '<option value="' + attr + '"' + selected + '>' + attr + '</option>';
                }).join('');
                keyHtml = '<select class="custom-attr-key w-1/3 bg-slate-950 border border-slate-700 rounded p-1.5 text-xs text-cyan-300 font-mono focus:outline-none focus:border-blue-500">' +
                    '<option value="" ' + (currentName ? '' : 'selected') + '>Select...</option>' +
                    opts +
                    '</select>';
            } else {
                const safeName = currentName.replace(/"/g, '&quot;');
                keyHtml = '<input type="text" placeholder="attribute_name" value="' + safeName + '" class="custom-attr-key w-1/3 bg-slate-950 border border-slate-700 rounded p-1.5 text-xs text-cyan-300 font-mono focus:outline-none focus:border-blue-500">';
            }

            const inputValue = currentIsAllowed || attrList.length === 0 ? safeVal : '';
            row.innerHTML = keyHtml +
                '<input type="text" placeholder="value" value="' + inputValue + '" class="custom-attr-val flex-1 bg-slate-950 border border-slate-700 rounded p-1.5 text-xs text-white focus:outline-none focus:border-blue-500">' +
                '<button type="button" onclick="this.closest(\'.custom-attr-row\').remove()" class="p-1.5 hover:bg-red-950/60 text-slate-400 hover:text-red-400 rounded text-xs" title="Remove attribute">&times;</button>';
            container.appendChild(row);
        }

        function getDefinedDatabases() {
            const dbNames = new Set();
            const dbSection = document.getElementById('nodes-databases');
            if (dbSection) {
                dbSection.querySelectorAll('.node-card').forEach(card => {
                    const name = card.dataset.name;
                    if (name && name.trim()) dbNames.add(name.trim());
                });
            }
            return Array.from(dbNames);
        }

        function renderModalFields(meta, currentValues) {
            window.currentNodeMeta = meta || getCurrentNodeMeta();
            const container = document.getElementById('modal-fields-container');
            container.innerHTML = '';
            const recognized = new Set();
            const definedDbs = getDefinedDatabases();

            (meta.fields || []).forEach(f => {
                recognized.add(f.name);
                const val = currentValues[f.name] !== undefined ? currentValues[f.name] : (f.default || '');
                const div = document.createElement('div');
                div.className = 'space-y-1';
                let inputHtml = '';
                if (f.name === 'db') {
                    // For database references, render a datalist with pulldown selection + free typing
                    let optionsHtml = '<option value="">-- Select Defined Database --</option>';
                    definedDbs.forEach(d => {
                        optionsHtml += '<option value="' + d + '" ' + (d === val ? 'selected' : '') + '>' + d + '</option>';
                    });
                    if (val && !definedDbs.includes(val)) {
                        optionsHtml += '<option value="' + val + '" selected>' + val + ' (custom)</option>';
                    }
                    inputHtml = '<div class="space-y-1">' +
                        '<select id="db-select-' + f.name + '" onchange="document.getElementById(\'db-input-' + f.name + '\').value = this.value" class="w-full bg-slate-950 border border-slate-700 rounded p-1.5 text-xs text-white focus:outline-none focus:border-blue-500">' +
                            optionsHtml +
                        '</select>' +
                        '<input type="text" id="db-input-' + f.name + '" name="attr_' + f.name + '" value="' + (val || '').replace(/"/g, '&quot;') + '" placeholder="Or enter database connection name..." oninput="document.getElementById(\'db-select-' + f.name + '\').value = this.value" class="w-full bg-slate-950 border border-slate-700/60 rounded p-1.5 text-xs text-cyan-300 font-mono focus:outline-none focus:border-blue-500">' +
                    '</div>';
                } else if (f.type === 'select') {
                    const opts = (f.options || []).map(o => '<option value="' + o + '" ' + (o === val ? 'selected' : '') + '>' + o + '</option>').join('');
                    inputHtml = '<select name="attr_' + f.name + '" class="w-full bg-slate-950 border border-slate-700 rounded p-1.5 text-xs text-white focus:outline-none focus:border-blue-500">' + opts + '</select>';
                } else {
                    inputHtml = '<input type="text" name="attr_' + f.name + '" value="' + (val || '').replace(/"/g, '&quot;') + '" class="w-full bg-slate-950 border border-slate-700 rounded p-1.5 text-xs text-white focus:outline-none focus:border-blue-500">';
                }
                div.innerHTML = '<div class="flex justify-between items-center"><label class="text-xs font-medium text-slate-300">' + f.label + (f.mandatory ? ' <span class="text-red-400">*</span>' : '') + '</label><span class="text-[10px] text-slate-500">' + (f.description || '') + '</span></div>' + inputHtml;
                container.appendChild(div);
            });

            // Populate custom/extra attributes not explicitly defined in meta.fields
            const customContainer = document.getElementById('modal-custom-attrs');
            customContainer.innerHTML = '';
            const allowedCustomAttrs = getAllowedCustomAttributeNames(meta);
            Object.keys(currentValues).forEach(k => {
                if (!recognized.has(k) && allowedCustomAttrs.includes(k)) {
                    addCustomAttributeRow(k, currentValues[k], allowedCustomAttrs);
                }
            });

            const contentBox = document.getElementById('modal-content-container');
            if (meta.has_content) {
                contentBox.classList.remove('hidden');
                document.getElementById('modal-content-label').textContent = meta.content_help || 'Content Body';
            } else {
                contentBox.classList.add('hidden');
                document.getElementById('modal-content-text').value = '';
            }
        }

        function closeNodeModal() {
            document.getElementById('node-modal').classList.add('hidden');
        }

        function submitNodeModal(e) {
            e.preventDefault();
            const form = e.target;
            const nodeId = form.querySelector('#modal-node-id').value;
            const nodeType = form.querySelector('#modal-node-type').value;
            const section = form.querySelector('#modal-node-section').value;
            const contentBox = document.getElementById('modal-content-container');
            const meta = catalogData.find(c => c.type === nodeType);
            const content = (!contentBox.classList.contains('hidden') && meta && meta.has_content)
                ? form.querySelector('#modal-content-text').value
                : '';
            const attrs = {};

            // Allowed predefined catalog fields
            new FormData(form).forEach((val, key) => {
                if (key.startsWith('attr_')) {
                    const attrName = key.replace('attr_', '');
                    if (val.trim() !== '') attrs[attrName] = val;
                }
            });

            const allowedCustomAttrs = getAllowedCustomAttributeNames(meta || getCurrentNodeMeta());
            form.querySelectorAll('.custom-attr-row').forEach(row => {
                const keyEl = row.querySelector('.custom-attr-key');
                const valEl = row.querySelector('.custom-attr-val');
                if (!keyEl || !valEl) return;
                const k = keyEl.value.trim();
                const v = valEl.value;
                if (k !== '' && allowedCustomAttrs.includes(k)) {
                    attrs[k] = v;
                }
            });

            const isEdit = nodeId !== '';
            const endpoint = isEdit ? '/api/nodes/update' : '/api/nodes/add';
            const body = {
                script_id: currentScriptId,
                node_id: isEdit ? parseInt(nodeId) : 0,
                node_type: nodeType,
                section: section,
                attributes: attrs,
                content: content
            };

            fetch(endpoint, {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify(body)
            }).then(() => {
                closeNodeModal();
                htmx.trigger(document.body, 'refreshCanvas');
                setTimeout(updatePreview, 100);
            }).catch(err => {
                alert('Save failed: ' + err);
            });
        }

        function deleteNode(id) {
            if (!confirm('Remove this step?')) return;
            fetch('/api/nodes/delete?id=' + id, {method: 'POST'})
                .then(() => {
                    htmx.trigger(document.body, 'refreshCanvas');
                    setTimeout(updatePreview, 100);
                });
        }

        function moveNode(id, dir) {
            fetch('/api/nodes/move?id=' + id + '&dir=' + dir, {method: 'POST'})
                .then(() => {
                    htmx.trigger(document.body, 'refreshCanvas');
                    setTimeout(updatePreview, 100);
                });
        }

        function saveToFileOnDisk() {
            const filename = prompt('Save pipeline XML to file path:', 'scripts.xml');
            if (!filename) return;
            fetch('/api/save_file', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({script_id: currentScriptId, filename: filename})
            }).then(r => r.text()).then(msg => alert(msg));
        }

        function downloadActiveXML() {
            fetch('/api/preview?script_id=' + currentScriptId)
                .then(r => r.text())
                .then(xml => {
                    const blob = new Blob([xml], {type: 'application/xml'});
                    const a = document.createElement('a');
                    a.href = URL.createObjectURL(blob);
                    a.download = 'scripts.xml';
                    a.click();
                });
        }

        function copyXMLPreview() {
            const text = document.getElementById('live-xml-preview').textContent;
            navigator.clipboard.writeText(text).then(() => alert('XML copied to clipboard!'));
        }

        function saveConfigFile() {
            const content = document.getElementById('config-editor').value;
            fetch('/api/config/save', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({name: 'CONFIG.xml', content: content})
            }).then(() => alert('CONFIG.xml saved successfully!'));
        }

        function saveOptionsFile() {
            const content = document.getElementById('options-editor').value;
            fetch('/api/options/save', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({name: 'options.xml', content: content})
            }).then(() => alert('options.xml saved successfully!'));
        }

        function submitNewScript(e) {
            e.preventDefault();
            const name = document.getElementById('new-script-name').value;
            const desc = document.getElementById('new-script-desc').value;
            fetch('/api/scripts/new', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({name: name, description: desc})
            }).then(r => r.json()).then(sc => {
                window.location.href = '/?script_id=' + sc.id;
            });
        }

        function openCopyScriptModal() {
            const select = document.getElementById('active-script-select');
            let currentName = 'pipeline';
            if (select && select.selectedOptions && select.selectedOptions[0]) {
                currentName = select.selectedOptions[0].text;
            }
            const nameInput = document.getElementById('copy-script-name');
            if (nameInput) {
                nameInput.value = currentName + ' (Copy)';
            }
            document.getElementById('copy-pipeline-modal').classList.remove('hidden');
            setTimeout(function() { if (nameInput) nameInput.focus(); }, 50);
        }

        function submitCopyScript(e) {
            e.preventDefault();
            const newName = document.getElementById('copy-script-name').value.trim();
            fetch('/api/scripts/copy', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({id: currentScriptId, new_name: newName})
            }).then(function(r) {
                if (!r.ok) return r.text().then(function(t) { throw new Error(t); });
                return r.json();
            }).then(function(res) {
                if (res.script && res.script.id) {
                    window.location.href = '/?script_id=' + res.script.id;
                } else {
                    window.location.reload();
                }
            }).catch(function(err) {
                alert('Failed to copy pipeline: ' + err.message);
            });
        }

        function openImportScriptModal() {
            document.getElementById('import-file-path').value = '';
            document.getElementById('import-script-name').value = '';
            document.getElementById('import-pipeline-modal').classList.remove('hidden');
        }

        function browseForImportFile() {
            openFileBrowser('import-file-path', '.xml');
        }

        function submitImportScript(e) {
            e.preventDefault();
            const filePath = document.getElementById('import-file-path').value.trim();
            const customName = document.getElementById('import-script-name').value.trim();
            if (!filePath) {
                alert('Please select or enter an XML pipeline file path');
                return;
            }
            fetch('/api/scripts/import', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({file_path: filePath, name: customName})
            }).then(function(r) {
                if (!r.ok) return r.text().then(function(t) { throw new Error(t); });
                return r.json();
            }).then(function(res) {
                if (res.script && res.script.id) {
                    window.location.href = '/?script_id=' + res.script.id;
                } else {
                    window.location.reload();
                }
            }).catch(function(err) {
                alert('Import failed: ' + err.message);
            });
        }

        function confirmDeleteScript() {
            const select = document.getElementById('active-script-select');
            let currentName = 'Active Pipeline';
            if (select && select.selectedOptions && select.selectedOptions[0]) {
                currentName = select.selectedOptions[0].text;
            }
            const nameEl = document.getElementById('delete-pipeline-name');
            if (nameEl) {
                nameEl.textContent = '"' + currentName + '"';
            }
            document.getElementById('delete-pipeline-modal').classList.remove('hidden');
        }

        function submitDeleteScript() {
            fetch('/api/scripts/delete?id=' + currentScriptId, {
                method: 'POST'
            }).then(function(r) {
                if (!r.ok) return r.text().then(function(t) { throw new Error(t); });
                return r.json();
            }).then(function(res) {
                if (res.next_script_id) {
                    window.location.href = '/?script_id=' + res.next_script_id;
                } else {
                    window.location.href = '/';
                }
            }).catch(function(err) {
                alert('Failed to delete pipeline: ' + err.message);
            });
        }

        function confirmPurgeDatabase() {
            document.getElementById('purge-db-modal').classList.remove('hidden');
        }

        function submitPurgeDatabase() {
            fetch('/api/db/purge', {
                method: 'POST'
            }).then(function(r) {
                if (!r.ok) return r.text().then(function(t) { throw new Error(t); });
                return r.json();
            }).then(function(res) {
                alert(res.message || 'Database purged successfully!');
                window.location.href = '/';
            }).catch(function(err) {
                alert('Purge failed: ' + err.message);
            });
        }

        let runnerSource = 'file'; // 'file' or 'builder'
        function setRunnerSource(src) {
            runnerSource = src;
            const btnFile = document.getElementById('source-btn-file');
            const btnBuilder = document.getElementById('source-btn-builder');
            const sourceLabel = document.getElementById('runner-source-label');
            const scriptInput = document.getElementById('runner-script-file');

            if (src === 'builder') {
                btnBuilder.className = 'px-3 py-1 text-xs font-semibold rounded-md transition-colors bg-blue-600 text-white shadow';
                btnFile.className = 'px-3 py-1 text-xs font-semibold rounded-md transition-colors text-slate-400 hover:text-white';
                sourceLabel.textContent = 'source: builder draft';
                sourceLabel.className = 'text-[10px] text-amber-400 font-mono';
                if (!scriptInput.value || scriptInput.value === 'scripts.xml') {
                    scriptInput.value = 'temp_run_script.xml';
                }
            } else {
                btnFile.className = 'px-3 py-1 text-xs font-semibold rounded-md transition-colors bg-blue-600 text-white shadow';
                btnBuilder.className = 'px-3 py-1 text-xs font-semibold rounded-md transition-colors text-slate-400 hover:text-white';
                sourceLabel.textContent = 'source: filesystem';
                sourceLabel.className = 'text-[10px] text-cyan-400 font-mono';
                if (scriptInput.value === 'temp_run_script.xml') {
                    scriptInput.value = 'scripts.xml';
                }
            }
        }

        function syncScriptWithOptionsSelection() {
            const scriptInput = document.getElementById('runner-script-file');
            const optionsInput = document.getElementById('runner-options-file');
            if (!scriptInput || !optionsInput) return;

            const scriptValue = scriptInput.value.trim();
            const optionsValue = optionsInput.value.trim();
            if (!optionsValue) return;

            if (!scriptValue || scriptValue === 'scripts.xml' || scriptValue === 'temp_run_script.xml') {
                scriptInput.value = '';
            }
        }

        document.addEventListener('DOMContentLoaded', function() {
            const optionsInput = document.getElementById('runner-options-file');
            if (optionsInput) {
                optionsInput.addEventListener('change', syncScriptWithOptionsSelection);
                optionsInput.addEventListener('input', function() {
                    if (this.value.trim() !== '') {
                        syncScriptWithOptionsSelection();
                    }
                });
            }
        });

        let browserTargetInputId = '';
        let browserFilterExt = '.xml';
        let browserCurrentDir = '.';
        let browserParentDir = '';
        let browseCacheEntries = [];

        function openFileBrowser(targetId, ext) {
            browserTargetInputId = targetId;
            browserFilterExt = ext || '';
            const modal = document.getElementById('file-browser-modal');
            const title = document.getElementById('file-browser-title');
            const subtitle = document.getElementById('file-browser-subtitle');

            if (targetId === 'runner-script-file') {
                title.textContent = 'Select Pipeline Script XML';
                subtitle.textContent = 'Choose an executable pipeline definition (.xml) from the filesystem';
            } else if (targetId === 'runner-config-file') {
                title.textContent = 'Select Config Overrides XML';
                subtitle.textContent = 'Choose a variable / database overrides file (CONFIG.xml)';
            } else if (targetId === 'runner-options-file') {
                title.textContent = 'Select CLI Options XML';
                subtitle.textContent = 'Choose CLI options XML file (options.xml)';
            } else {
                title.textContent = 'Select File';
                subtitle.textContent = 'Choose a file from the filesystem';
            }

            modal.classList.remove('hidden');
            loadDirectory(browserCurrentDir);
        }

        function closeFileBrowser() {
            const modal = document.getElementById('file-browser-modal');
            if (modal) modal.classList.add('hidden');
            const filterInput = document.getElementById('file-browser-filter');
            if (filterInput) filterInput.value = '';
        }

        function loadDirectory(dir) {
            browserCurrentDir = dir;
            const url = '/api/files/browse?dir=' + encodeURIComponent(dir) + '&ext=' + encodeURIComponent(browserFilterExt);
            fetch(url)
                .then(r => {
                    if (!r.ok) throw new Error('Failed to read directory');
                    return r.json();
                })
                .then(data => {
                    browserCurrentDir = data.current_dir;
                    browserParentDir = data.parent_dir;
                    document.getElementById('file-browser-current-path').textContent = data.current_dir || '.';
                    const upBtn = document.getElementById('file-browser-up-btn');
                    if (data.parent_dir && data.parent_dir !== data.current_dir) {
                        upBtn.disabled = false;
                        upBtn.classList.remove('opacity-40', 'cursor-not-allowed');
                    } else {
                        upBtn.disabled = true;
                        upBtn.classList.add('opacity-40', 'cursor-not-allowed');
                    }
                    browseCacheEntries = data.entries || [];
                    renderBrowseEntries(browseCacheEntries);
                })
                .catch(err => {
                    document.getElementById('file-browser-list').innerHTML =
                        '<div class="p-4 text-center text-red-400 text-xs">Error reading directory: ' + err.message + '</div>';
                });
        }

        function browseUp() {
            if (browserParentDir) {
                loadDirectory(browserParentDir);
            }
        }

        function refreshFileBrowser() {
            loadDirectory(browserCurrentDir);
        }

        function filterBrowseResults(query) {
            const q = query.trim().toLowerCase();
            if (!q) {
                renderBrowseEntries(browseCacheEntries);
                return;
            }
            const filtered = browseCacheEntries.filter(e => e.name.toLowerCase().includes(q));
            renderBrowseEntries(filtered);
        }

        function formatBytes(bytes) {
            if (bytes === 0) return '0 B';
            const k = 1024;
            const sizes = ['B', 'KB', 'MB', 'GB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
        }

        function renderBrowseEntries(entries) {
            const listEl = document.getElementById('file-browser-list');
            const countEl = document.getElementById('file-browser-count-text');
            countEl.textContent = entries.length + ' item' + (entries.length === 1 ? '' : 's');

            if (entries.length === 0) {
                listEl.innerHTML = '<div class="p-8 text-center text-slate-500 text-xs">No files or subdirectories found</div>';
                return;
            }

            let html = '<div class="divide-y divide-slate-800/60">';
            entries.forEach(e => {
                const safePath = e.path.replace(/\\/g, '\\\\').replace(/'/g, "\\'");
                if (e.is_dir) {
                    html += '<div onclick="loadDirectory(\'' + safePath + '\')" class="p-2 hover:bg-slate-800/70 rounded cursor-pointer flex items-center justify-between text-xs transition group">' +
                        '<div class="flex items-center space-x-2 text-amber-300 font-medium truncate">' +
                            '<span>📁</span>' +
                            '<span class="group-hover:text-amber-200">' + e.name + '</span>' +
                        '</div>' +
                        '<span class="text-[10px] text-slate-500 font-mono">DIR</span>' +
                    '</div>';
                } else {
                    html += '<div onclick="selectFileFromBrowser(\'' + safePath + '\')" class="p-2 hover:bg-blue-600/20 hover:border-blue-500/40 border border-transparent rounded cursor-pointer flex items-center justify-between text-xs transition group">' +
                        '<div class="flex items-center space-x-2 text-slate-200 font-mono truncate">' +
                            '<span class="text-emerald-400">📄</span>' +
                            '<span class="group-hover:text-cyan-300 font-medium">' + e.name + '</span>' +
                        '</div>' +
                        '<div class="flex items-center space-x-3 text-[10px] text-slate-400 font-mono">' +
                            '<span>' + formatBytes(e.size) + '</span>' +
                            '<button type="button" class="px-2 py-0.5 rounded bg-blue-600/70 text-white group-hover:bg-blue-500 text-[10px]">Select</button>' +
                        '</div>' +
                    '</div>';
                }
            });
            html += '</div>';
            listEl.innerHTML = html;
        }

        function selectFileFromBrowser(filePath) {
            if (browserTargetInputId) {
                const target = document.getElementById(browserTargetInputId);
                if (target) {
                    target.value = filePath;
                    if (browserTargetInputId === 'runner-script-file') {
                        setRunnerSource('file');
                    }
                    if (browserTargetInputId === 'runner-options-file') {
                        syncScriptWithOptionsSelection();
                    }
                    if (browserTargetInputId === 'import-file-path') {
                        const nameInput = document.getElementById('import-script-name');
                        if (nameInput && !nameInput.value.trim()) {
                            const parts = filePath.replace(/\\/g, '/').split('/');
                            const fileName = parts[parts.length - 1];
                            const baseName = fileName.replace(/\.[^/.]+$/, '');
                            nameInput.placeholder = baseName;
                        }
                    }
                }
            }
            closeFileBrowser();
        }

        function populateQuickFilesList() {
            fetch('/api/files/quick?ext=.xml')
                .then(r => r.json())
                .then(files => {
                    const dl = document.getElementById('quick-xml-files');
                    if (dl && Array.isArray(files)) {
                        dl.innerHTML = '';
                        files.forEach(f => {
                            const opt = document.createElement('option');
                            opt.value = f;
                            dl.appendChild(opt);
                        });
                    }
                })
                .catch(() => {});
        }

        function escapeHtml(str) {
            if (!str) return '';
            return String(str)
                .replace(/&/g, '&amp;')
                .replace(/</g, '&lt;')
                .replace(/>/g, '&gt;')
                .replace(/"/g, '&quot;');
        }

        function handleNodeEvent(evt, nodeStartTimes, tbody) {
            const evtType = (evt.event_type || '').toLowerCase().replace(/_/g, '.');
            if (evtType === 'run.started' || evtType === 'run.finished') {
                return;
            }
            const nodeId = evt.node_id || evt.execution_id || 'unknown';
            const rowId = 'node-row-' + nodeId.replace(/[^a-zA-Z0-9_-]/g, '_');

            if (evtType === 'node.started') {
                nodeStartTimes[nodeId] = Date.now();
                let row = document.getElementById(rowId);
                if (!row) {
                    row = document.createElement('tr');
                    row.id = rowId;
                    row.className = 'hover:bg-slate-800/40 transition-colors';
                    tbody.appendChild(row);
                }
                row.innerHTML =
                    '<td class="p-2.5 font-bold text-white flex items-center space-x-1.5">' +
                        '<span class="text-xs">⚡</span>' +
                        '<span>' + escapeHtml(nodeId) + '</span>' +
                    '</td>' +
                    '<td class="p-2.5 text-slate-400">' +
                        '<span class="px-2 py-0.5 rounded text-[10px] bg-slate-800 border border-slate-700 uppercase font-mono">' + escapeHtml(evt.kind || 'node') + '</span>' +
                    '</td>' +
                    '<td class="p-2.5" id="' + rowId + '-status">' +
                        '<span class="px-2 py-0.5 rounded text-[10px] bg-amber-500/20 text-amber-400 border border-amber-500/30 animate-pulse font-semibold">RUNNING</span>' +
                    '</td>' +
                    '<td class="p-2.5 text-slate-500" id="' + rowId + '-rc">-</td>' +
                    '<td class="p-2.5 text-slate-400 font-mono" id="' + rowId + '-dur">running...</td>' +
                    '<td class="p-2.5 text-slate-400 truncate max-w-xs font-mono text-[10px]" id="' + rowId + '-out">Executing operation...</td>';
                return;
            }

            if (evtType === 'node.finished') {
                let row = document.getElementById(rowId);
                if (!row) {
                    row = document.createElement('tr');
                    row.id = rowId;
                    row.className = 'hover:bg-slate-800/40 transition-colors';
                    tbody.appendChild(row);
                    row.innerHTML =
                        '<td class="p-2.5 font-bold text-white flex items-center space-x-1.5">' +
                            '<span class="text-xs">⚡</span>' +
                            '<span>' + escapeHtml(nodeId) + '</span>' +
                        '</td>' +
                        '<td class="p-2.5 text-slate-400">' +
                            '<span class="px-2 py-0.5 rounded text-[10px] bg-slate-800 border border-slate-700 uppercase font-mono">' + escapeHtml(evt.kind || 'node') + '</span>' +
                        '</td>' +
                        '<td class="p-2.5" id="' + rowId + '-status"></td>' +
                        '<td class="p-2.5" id="' + rowId + '-rc"></td>' +
                        '<td class="p-2.5 font-mono" id="' + rowId + '-dur"></td>' +
                        '<td class="p-2.5 truncate max-w-xs font-mono text-[10px]" id="' + rowId + '-out"></td>';
                }
                const isSuccess = evt.status === 'succeeded' || evt.status === 'success';
                const statusCell = document.getElementById(rowId + '-status');
                const rcCell = document.getElementById(rowId + '-rc');
                const durCell = document.getElementById(rowId + '-dur');
                const outCell = document.getElementById(rowId + '-out');

                if (statusCell) {
                    statusCell.innerHTML = isSuccess
                        ? '<span class="px-2 py-0.5 rounded text-[10px] bg-emerald-500/20 text-emerald-400 border border-emerald-500/30 font-semibold">SUCCEEDED</span>'
                        : '<span class="px-2 py-0.5 rounded text-[10px] bg-red-500/20 text-red-400 border border-red-500/30 font-semibold">FAILED</span>';
                }
                if (rcCell) {
                    rcCell.className = isSuccess ? 'p-2.5 text-emerald-400 font-bold' : 'p-2.5 text-red-400 font-bold';
                    rcCell.textContent = isSuccess ? '0' : '1';
                }
                if (durCell) {
                    const sTime = nodeStartTimes[nodeId];
                    if (sTime) {
                        const durMs = Date.now() - sTime;
                        durCell.textContent = durMs < 1000 ? durMs + 'ms' : (durMs / 1000).toFixed(2) + 's';
                    } else {
                        durCell.textContent = '0ms';
                    }
                }
                if (outCell) {
                    if (!isSuccess && evt.error) {
                        outCell.className = 'p-2.5 text-red-400 truncate max-w-xs font-mono text-[10px]';
                        outCell.title = evt.error;
                        outCell.textContent = evt.error;
                    } else {
                        outCell.className = 'p-2.5 text-slate-300 truncate max-w-xs font-mono text-[10px]';
                        const parts = [];
                        if (evt.rows_read) parts.push('Read: ' + Number(evt.rows_read).toLocaleString());
                        if (evt.rows_written) parts.push('Written: ' + Number(evt.rows_written).toLocaleString());
                        if (evt.rows_affected) parts.push('Affected: ' + Number(evt.rows_affected).toLocaleString());
                        const summary = parts.length > 0 ? parts.join(' | ') : 'Completed successfully';
                        outCell.textContent = summary;
                        outCell.title = summary;
                    }
                }
            }
        }

        let eventSource = null;
        function startPipelineExecution() {
            const scriptInput = document.getElementById('runner-script-file');
            const scriptFile = scriptInput ? scriptInput.value.trim() : '';
            const configFile = document.getElementById('runner-config-file').value.trim();
            const optionsFile = document.getElementById('runner-options-file').value.trim();
            const term = document.getElementById('terminal-log');
            const tbody = document.getElementById('execution-results-body');
            const statusInd = document.getElementById('status-indicator');
            const statusText = document.getElementById('status-text');
            const timerEl = document.getElementById('status-timer');

            const shouldIncludeScript = runnerSource === 'builder' || scriptFile !== '' || optionsFile === '';

            term.textContent = '';
            tbody.innerHTML = '';
            statusInd.className = 'h-2.5 w-2.5 rounded-full bg-amber-400 animate-ping';
            statusText.textContent = 'Execution in progress...';
            const startTime = Date.now();
            const nodeStartTimes = {};
            const timerInterval = setInterval(() => {
                const diff = (Date.now() - startTime) / 1000;
                timerEl.textContent = diff.toFixed(3) + 's';
            }, 100);

            if (eventSource) eventSource.close();

            let url = '/api/execute/stream?source=' + encodeURIComponent(runnerSource) +
                        '&script_id=' + currentScriptId +
                        '&config=' + encodeURIComponent(configFile) +
                        '&options=' + encodeURIComponent(optionsFile);
            if (shouldIncludeScript) {
                url += '&file=' + encodeURIComponent(scriptFile || (runnerSource === 'builder' ? 'temp_run_script.xml' : 'scripts.xml'));
            }
            eventSource = new EventSource(url);

            eventSource.onmessage = function(e) {
                const data = JSON.parse(e.data);
                if (data.type === 'log') {
                    term.textContent += data.message + '\n';
                    term.scrollTop = term.scrollHeight;
                } else if (data.type === 'node_event') {
                    handleNodeEvent(data, nodeStartTimes, tbody);
                } else if (data.type === 'done') {
                    clearInterval(timerInterval);
                    eventSource.close();
                    if (tbody.children.length === 0) {
                        tbody.innerHTML = '<tr><td colspan="6" class="p-4 text-center text-slate-500">' +
                            (data.error ? ('Execution stopped: ' + escapeHtml(data.error)) : 'Execution completed with no node events.') +
                            '</td></tr>';
                    }
                    if (data.status === 'SUCCESS') {
                        statusInd.className = 'h-2.5 w-2.5 rounded-full bg-emerald-500';
                        statusText.textContent = 'Execution Finished Successfully (' + data.duration + ')';
                    } else {
                        statusInd.className = 'h-2.5 w-2.5 rounded-full bg-red-500';
                        statusText.textContent = 'Execution Failed: ' + (data.error || 'Error');
                    }
                }
            };
            eventSource.onerror = function() {
                clearInterval(timerInterval);
                statusInd.className = 'h-2.5 w-2.5 rounded-full bg-red-500';
                statusText.textContent = 'Connection closed or lost.';
                if (eventSource) eventSource.close();
            };
        }

        function setTheme(theme) {
            document.documentElement.setAttribute('data-theme', theme);
            localStorage.setItem('flow_builder_theme', theme);
            const sel = document.getElementById('theme-selector');
            if (sel) sel.value = theme;
        }

        function loadSavedTheme() {
            const saved = localStorage.getItem('flow_builder_theme') || 'slate';
            setTheme(saved);
        }

        document.addEventListener('keydown', (e) => {
            if (e.key === 'Escape') {
                closeFileBrowser();
                closeNodeModal();
                closeComponentPicker();
            }
        });

        function highlightSectionNav(secKey) {
            const sections = ['variables', 'databases', 'preflight', 'flow'];
            sections.forEach(function(s) {
                const link = document.getElementById('nav-section-' + s);
                if (!link) return;
                if (s === secKey) {
                    link.className = 'section-nav-link px-2.5 py-1 bg-blue-900/60 text-blue-300 font-semibold rounded border border-blue-500/80 shadow-sm transition';
                } else {
                    link.className = 'section-nav-link px-2.5 py-1 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded border border-slate-700 transition';
                }
            });
        }

        let isProgrammaticScroll = false;
        let programmaticScrollTimer = null;

        function navigateToSection(secKey, e) {
            if (e && e.preventDefault) e.preventDefault();
            const target = document.getElementById('section-' + secKey);
            const canvas = document.getElementById('canvas-content');
            if (target && canvas) {
                isProgrammaticScroll = true;
                clearTimeout(programmaticScrollTimer);
                const targetRect = target.getBoundingClientRect();
                const canvasRect = canvas.getBoundingClientRect();
                const scrollDelta = targetRect.top - canvasRect.top - 16;
                canvas.scrollBy({ top: scrollDelta, behavior: 'smooth' });
                programmaticScrollTimer = setTimeout(function() {
                    isProgrammaticScroll = false;
                }, 600);
            }
            if (window.history && window.history.pushState) {
                window.history.pushState(null, null, '#section-' + secKey);
            } else {
                window.location.hash = '#section-' + secKey;
            }
            highlightSectionNav(secKey);
        }

        function updateActiveSectionFromScroll() {
            if (isProgrammaticScroll) return;
            const canvas = document.getElementById('canvas-content');
            if (!canvas) return;

            const sections = ['variables', 'databases', 'preflight', 'flow'];
            const canvasRect = canvas.getBoundingClientRect();

            // If scrolled near bottom of canvas, activate the last section (flow)
            if (canvas.scrollHeight - canvas.scrollTop - canvas.clientHeight < 40) {
                highlightSectionNav('flow');
                return;
            }

            let active = 'variables';
            for (let i = 0; i < sections.length; i++) {
                const s = sections[i];
                const el = document.getElementById('section-' + s);
                if (el) {
                    const elRect = el.getBoundingClientRect();
                    if (elRect.top - canvasRect.top <= 140) {
                        active = s;
                    }
                }
            }
            highlightSectionNav(active);
        }

        let sectionScrollDebounceTimer = null;
        function handleCanvasScroll() {
            if (sectionScrollDebounceTimer) return;
            sectionScrollDebounceTimer = setTimeout(function() {
                sectionScrollDebounceTimer = null;
                updateActiveSectionFromScroll();
            }, 50);
        }

        function initSectionNavigation() {
            const canvas = document.getElementById('canvas-content');
            if (canvas) {
                canvas.removeEventListener('scroll', handleCanvasScroll);
                canvas.addEventListener('scroll', handleCanvasScroll, { passive: true });
            }

            const rawHash = (window.location.hash || '').replace('#section-', '').replace('#', '');
            if (rawHash && ['variables', 'databases', 'preflight', 'flow'].includes(rawHash)) {
                highlightSectionNav(rawHash);
                setTimeout(function() {
                    navigateToSection(rawHash);
                }, 100);
            } else {
                updateActiveSectionFromScroll();
            }
        }

        window.addEventListener('hashchange', function() {
            const rawHash = (window.location.hash || '').replace('#section-', '').replace('#', '');
            if (rawHash && ['variables', 'databases', 'preflight', 'flow'].includes(rawHash)) {
                navigateToSection(rawHash);
            }
        });

        document.addEventListener('DOMContentLoaded', () => {
            loadSavedTheme();
            initSortables();
            updatePreview();
            populateQuickFilesList();
            initSectionNavigation();
        });
        document.body.addEventListener('htmx:afterSwap', () => {
            initSortables();
            updatePreview();
            initSectionNavigation();
        });
    </script>
</body>

</html>
`
