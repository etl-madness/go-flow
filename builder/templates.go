package builder

import (
	"encoding/json"
	"html/template"
)

const ScriptSelectorPartial = `{{define "script_selector"}}
<select id="active-script-select" onchange="window.location.href='/?script_id=' + this.value"
        class="bg-slate-950 border border-slate-700 rounded-md px-3 py-1.5 text-xs text-cyan-300 font-semibold focus:outline-none focus:border-blue-500 shadow-inner">
    {{range .AllScripts}}
    <option value="{{.ID}}" {{if eq .ID $.ActiveScript.ID}}selected{{end}}>{{.Name}}</option>
    {{end}}
</select>
<button onclick="document.getElementById('new-script-modal').classList.remove('hidden')" 
        class="px-2.5 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 rounded-md text-xs font-semibold transition" title="Create New Script">
    ＋ New
</button>
{{end}}`

const CanvasNodesPartial = `{{define "canvas_nodes"}}
<div class="space-y-6">
    <!-- 1. Variables Section -->
    <div class="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-sm" id="section-variables">
        <div class="flex items-center justify-between mb-4 border-b border-slate-800 pb-2.5">
            <div class="flex items-center space-x-2">
                <span class="text-sm font-bold text-slate-200">1. Variables</span>
                <span class="text-xs text-slate-400">(&lt;variables&gt;)</span>
            </div>
            <button onclick="openAddNodeModal('variable', 'Variable', 'variables')"
                    class="px-2.5 py-1 bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 rounded text-xs transition">
                ＋ Add Variable
            </button>
        </div>
        <div id="nodes-variables" class="space-y-2 min-h-[50px]">
            {{range .VariableNodes}}
                {{template "node_card" .}}
            {{else}}
                <div class="p-4 border border-dashed border-slate-800 rounded-lg text-center text-xs text-slate-400">
                    No variables defined yet.
                </div>
            {{end}}
        </div>
    </div>

    <!-- 2. Databases Section -->
    <div class="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-sm" id="section-databases">
        <div class="flex items-center justify-between mb-4 border-b border-slate-800 pb-2.5">
            <div class="flex items-center space-x-2">
                <span class="text-sm font-bold text-slate-200">2. Databases</span>
                <span class="text-xs text-slate-400">(&lt;databases&gt;)</span>
            </div>
            <button onclick="openAddNodeModal('database', 'Database Connection', 'databases')"
                    class="px-2.5 py-1 bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 rounded text-xs transition">
                ＋ Add Database
            </button>
        </div>
        <div id="nodes-databases" class="space-y-2 min-h-[50px]">
            {{range .DatabaseNodes}}
                {{template "node_card" .}}
            {{else}}
                <div class="p-4 border border-dashed border-slate-800 rounded-lg text-center text-xs text-slate-400">
                    No database connections defined.
                </div>
            {{end}}
        </div>
    </div>

    <!-- 3. Preflight Section -->
    <div class="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-sm" id="section-preflight">
        <div class="flex items-center justify-between mb-4 border-b border-slate-800 pb-2.5">
            <div class="flex items-center space-x-2">
                <span class="text-sm font-bold text-slate-200">3. Preflight Checks</span>
                <span class="text-xs text-slate-400">(&lt;preflight&gt;)</span>
            </div>
            <button onclick="openComponentPicker('preflight')"
                    class="px-2.5 py-1 bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 rounded text-xs transition">
                ＋ Add Preflight Step
            </button>
        </div>
        <div id="nodes-preflight" class="space-y-2 min-h-[50px]">
            {{range .PreflightNodes}}
                {{template "node_card" .}}
            {{else}}
                <div class="p-4 border border-dashed border-slate-800 rounded-lg text-center text-xs text-slate-400">
                    No preflight validation gates configured.
                </div>
            {{end}}
        </div>
    </div>

    <!-- 4. Main Flow Section -->
    <div class="bg-slate-900 border border-blue-900/40 rounded-xl p-5 shadow-sm" id="section-flow">
        <div class="flex items-center justify-between mb-4 border-b border-slate-800 pb-2.5">
            <div class="flex items-center space-x-2">
                <span class="text-sm font-bold text-blue-400">4. Pipeline Flow</span>
                <span class="text-xs text-slate-400">(&lt;flow&gt; sequential steps)</span>
            </div>
            <button onclick="openComponentPicker('flow')"
                    class="px-2.5 py-1 bg-blue-600 hover:bg-blue-500 text-white rounded text-xs transition shadow">
                ＋ Add Flow Step
            </button>
        </div>
        <div id="nodes-flow" class="space-y-2 min-h-[80px]">
            {{range .FlowNodes}}
                {{template "node_card" .}}
            {{else}}
                <div class="p-6 border border-dashed border-slate-800 rounded-lg text-center text-xs text-slate-400">
                    Select a component from the palette on the left to add steps into this pipeline.
                </div>
            {{end}}
        </div>
    </div>
</div>
{{end}}`

const NodeCardPartial = `{{define "node_card"}}
<div class="node-card bg-slate-950 border border-slate-800 hover:border-slate-700 rounded-lg p-3 flex items-center justify-between transition shadow-sm"
     data-node-id="{{.ID}}" data-section="{{.Section}}" data-type="{{.NodeType}}" {{if .Attributes.name}}data-name="{{.Attributes.name}}"{{end}}>
    <div class="flex items-center space-x-3 overflow-hidden">
        <div class="drag-handle cursor-grab active:cursor-grabbing text-slate-400 hover:text-slate-300 px-1 py-1 text-sm select-none" title="Drag to re-order sequence">
            ⋮⋮
        </div>
        <div class="flex flex-col min-w-0">
            <div class="flex items-center space-x-2">
                <span class="text-xs font-mono font-bold text-cyan-400">&lt;{{.NodeType}}&gt;</span>
                {{if .Attributes.name}}
                    <span class="text-xs font-semibold text-white truncate">{{.Attributes.name}}</span>
                {{else if .Attributes.id}}
                    <span class="text-xs font-semibold text-white truncate">{{.Attributes.id}}</span>
                {{end}}
            </div>
            <div class="text-[11px] font-mono text-slate-400 truncate max-w-md">
                {{range $k, $v := .Attributes}}
                    {{if and (ne $k "id") (ne $k "name")}}
                        <span class="text-slate-400">{{$k}}:</span> <span class="text-slate-300">{{$v}}</span> 
                    {{end}}
                {{end}}
                {{if .ContentText}}
                    <span class="text-cyan-500">[body]</span>
                {{end}}
            </div>
        </div>
    </div>
    <div class="flex items-center space-x-1.5 shrink-0 ml-3">
        <button onclick="moveNode({{.ID}}, 'up')" class="p-1 hover:bg-slate-800 text-slate-400 hover:text-white rounded text-xs" title="Move Up">▲</button>
        <button onclick="moveNode({{.ID}}, 'down')" class="p-1 hover:bg-slate-800 text-slate-400 hover:text-white rounded text-xs" title="Move Down">▼</button>
        <button onclick="openEditNode({{.ID}})" 
                class="px-2 py-0.5 bg-slate-800 hover:bg-slate-700 text-cyan-400 rounded text-xs font-medium">Edit</button>
        <button onclick="deleteNode({{.ID}})" class="p-1 hover:bg-red-950/50 text-slate-400 hover:text-red-400 rounded text-xs" title="Delete">✕</button>
    </div>
</div>
{{end}}`

func BuildTemplate() (*template.Template, error) {
	funcMap := template.FuncMap{
		"toJS": func(v any) template.JS {
			b, _ := json.Marshal(v)
			return template.JS(b)
		},
	}

	tmpl := template.New("index").Funcs(funcMap)
	var err error
	if tmpl, err = tmpl.Parse(IndexHTML); err != nil {
		return nil, err
	}
	if tmpl, err = tmpl.Parse(ScriptSelectorPartial); err != nil {
		return nil, err
	}
	if tmpl, err = tmpl.Parse(CanvasNodesPartial); err != nil {
		return nil, err
	}
	if tmpl, err = tmpl.Parse(NodeCardPartial); err != nil {
		return nil, err
	}
	return tmpl, nil
}