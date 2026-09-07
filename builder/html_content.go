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
        html[data-theme="light"] {
            filter: invert(93%) hue-rotate(180deg) contrast(95%);
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
                    <option value="light" class="bg-slate-900 text-slate-200">Clean Light</option>
                </select>
            </div>
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
                    <div class="flex items-center space-x-3 text-xs">
                        <span class="text-slate-400 font-medium">Pipeline Sections:</span>
                        <a href="#section-variables" class="px-2.5 py-1 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded border border-slate-700 transition">Variables</a>
                        <a href="#section-databases" class="px-2.5 py-1 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded border border-slate-700 transition">Databases</a>
                        <a href="#section-preflight" class="px-2.5 py-1 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded border border-slate-700 transition">Preflight</a>
                        <a href="#section-flow" class="px-2.5 py-1 bg-blue-900/40 text-blue-300 rounded border border-blue-700/50 transition">Main Flow</a>
                    </div>
                    <div class="text-xs text-slate-400">Drag cards or use ▲/▼ to re-order sequence</div>
                </div>
                <div class="flex-1 overflow-y-auto custom-scrollbar p-6 space-y-6" id="canvas-content"
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
                <div class="px-6 py-4 border-b border-slate-800 bg-slate-900/70 flex flex-wrap items-center justify-between gap-4">
                    <div>
                        <h2 class="text-base font-bold text-white">🚀 Execute Pipeline</h2>
                        <p class="text-xs text-slate-400">Stream execution events live to the webpage.</p>
                    </div>
                    <div class="flex items-center space-x-3">
                        <input type="text" id="runner-script-file" value="scripts.xml" placeholder="scripts.xml" class="bg-slate-950 border border-slate-700 rounded px-2 py-1 text-xs text-slate-200 w-32 focus:outline-none focus:border-blue-500">
                        <input type="text" id="runner-config-file" value="" placeholder="CONFIG.xml (optional)" class="bg-slate-950 border border-slate-700 rounded px-2 py-1 text-xs text-slate-200 w-36 focus:outline-none focus:border-blue-500">
                        <input type="text" id="runner-options-file" value="" placeholder="options.xml (optional)" class="bg-slate-950 border border-slate-700 rounded px-2 py-1 text-xs text-slate-200 w-36 focus:outline-none focus:border-blue-500">
                        <button onclick="startPipelineExecution()" id="execute-btn" class="px-5 py-2 text-xs font-bold bg-blue-600 hover:bg-blue-500 text-white rounded-lg transition shadow flex items-center space-x-1">
                            <span>▶ Run Execution</span>
                        </button>
                    </div>
                </div>
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
                        <button type="button" onclick="addCustomAttributeRow('', '')" class="text-[11px] text-cyan-400 hover:text-cyan-300 font-medium">＋ Add Attribute</button>
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
            if (tab === 'pipeline') updatePreview();
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

        function addCustomAttributeRow(name, value) {
            const container = document.getElementById('modal-custom-attrs');
            const row = document.createElement('div');
            row.className = 'flex items-center space-x-2 custom-attr-row';
            const safeName = (name || '').replace(/"/g, '&quot;');
            const safeVal = (value || '').replace(/"/g, '&quot;');
            row.innerHTML = '<input type="text" placeholder="attribute_name" value="' + safeName + '" class="custom-attr-key w-1/3 bg-slate-950 border border-slate-700 rounded p-1.5 text-xs text-cyan-300 font-mono focus:outline-none focus:border-blue-500">' +
                '<input type="text" placeholder="value" value="' + safeVal + '" class="custom-attr-val flex-1 bg-slate-950 border border-slate-700 rounded p-1.5 text-xs text-white focus:outline-none focus:border-blue-500">' +
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
            Object.keys(currentValues).forEach(k => {
                if (!recognized.has(k)) {
                    addCustomAttributeRow(k, currentValues[k]);
                }
            });

            const contentBox = document.getElementById('modal-content-container');
            if (meta.has_content) {
                contentBox.classList.remove('hidden');
                document.getElementById('modal-content-label').textContent = meta.content_help || 'Content Body';
            } else {
                contentBox.classList.add('hidden');
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
            const content = form.querySelector('#modal-content-text').value;
            const attrs = {};

            // Allowed predefined catalog fields
            new FormData(form).forEach((val, key) => {
                if (key.startsWith('attr_')) {
                    const attrName = key.replace('attr_', '');
                    if (val.trim() !== '') attrs[attrName] = val;
                }
            });

            // Custom / extra attributes
            form.querySelectorAll('.custom-attr-row').forEach(row => {
                const k = row.querySelector('.custom-attr-key').value.trim();
                const v = row.querySelector('.custom-attr-val').value;
                if (k !== '') {
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

        let eventSource = null;
        function startPipelineExecution() {
            const scriptFile = document.getElementById('runner-script-file').value;
            const configFile = document.getElementById('runner-config-file').value;
            const optionsFile = document.getElementById('runner-options-file').value;
            const term = document.getElementById('terminal-log');
            const tbody = document.getElementById('execution-results-body');
            const statusInd = document.getElementById('status-indicator');
            const statusText = document.getElementById('status-text');
            const timerEl = document.getElementById('status-timer');

            term.textContent = '';
            tbody.innerHTML = '';
            statusInd.className = 'h-2.5 w-2.5 rounded-full bg-amber-400 animate-ping';
            statusText.textContent = 'Execution in progress...';
            const startTime = Date.now();
            const timerInterval = setInterval(() => {
                const diff = (Date.now() - startTime) / 1000;
                timerEl.textContent = diff.toFixed(3) + 's';
            }, 100);

            if (eventSource) eventSource.close();
            const url = '/api/execute/stream?script_id=' + currentScriptId + '&file=' + encodeURIComponent(scriptFile) + '&config=' + encodeURIComponent(configFile) + '&options=' + encodeURIComponent(optionsFile);
            eventSource = new EventSource(url);

            eventSource.onmessage = function(e) {
                const data = JSON.parse(e.data);
                if (data.type === 'log') {
                    term.textContent += data.message + '\n';
                    term.scrollTop = term.scrollHeight;
                } else if (data.type === 'done') {
                    clearInterval(timerInterval);
                    eventSource.close();
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

        document.addEventListener('DOMContentLoaded', () => {
            loadSavedTheme();
            initSortables();
            updatePreview();
        });
        document.body.addEventListener('htmx:afterSwap', () => {
            initSortables();
            updatePreview();
        });
    </script>
</body>

</html>
`