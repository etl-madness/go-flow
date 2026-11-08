function sanitizeMermaidSource(src) {
    // Preserve <br/> and <br>, escape raw XML tags like <foreach> or <group ...> inside quoted labels
    const text = src.trim().replace(/<br\s*\/?>/gi, '___BR___');
    const nextBoundary = new Int32Array(text.length + 1);
    let boundary = text.length;
    for (let i = text.length - 1; i >= 0; i--) {
        if (text[i] === '>' || text[i] === '\n' || text[i] === '"') boundary = i;
        nextBoundary[i] = boundary;
    }
    nextBoundary[text.length] = text.length;

    const name = /\/?[a-zA-Z_][\w:.-]*/y;
    const whitespace = /\s+/y;
    const parts = [];
    let copied = 0;
    let i = 0;
    while (i < text.length) {
        if (text[i] !== '<') {
            i++;
            continue;
        }
        name.lastIndex = i + 1;
        if (!name.exec(text)) {
            i++;
            continue;
        }
        let end = name.lastIndex;
        if (text[end] !== '>') {
            whitespace.lastIndex = end;
            if (!whitespace.exec(text)) {
                i++;
                continue;
            }
            end = nextBoundary[whitespace.lastIndex];
        }
        if (text[end] !== '>') {
            i++;
            continue;
        }
        parts.push(text.slice(copied, i), '&lt;', text.slice(i + 1, end), '&gt;');
        copied = end + 1;
        i = copied;
    }
    parts.push(text.slice(copied));
    return parts.join('').replace(/___BR___/g, '<br/>');
}

if (typeof module !== 'undefined' && module.exports) {
    module.exports = sanitizeMermaidSource;
}
