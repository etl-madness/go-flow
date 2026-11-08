const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const sanitize = require('./sanitize_mermaid.js');

test('documentation page loads the shared sanitizer before using it', () => {
    const page = fs.readFileSync(path.join(__dirname, 'index.html'), 'utf8');
    const script = page.indexOf('<script src="sanitize_mermaid.js"></script>');
    const use = page.indexOf('sanitizeMermaidSource(codeEl.textContent)');
    assert.ok(script !== -1 && use > script);
});

test('preserves tag escaping and line breaks', () => {
    const cases = [
        ['flowchart TD\n  A["<foreach> and <group id=x>"]', 'flowchart TD\n  A["&lt;foreach&gt; and &lt;group id=x&gt;"]'],
        ['<br><BR /> <tag> </tag> <_tag a=b/>', '<br/><br/> &lt;tag&gt; &lt;/tag&gt; &lt;_tag a=b/&gt;'],
        ['<tag attr="value"> <1tag> <tag\n name=x> <tag a\nb>', '<tag attr="value"> <1tag> &lt;tag\n name=x&gt; <tag a\nb>'],
        ['<a <b> <c> <unfinished <nested value', '&lt;a <b&gt; &lt;c&gt; <unfinished <nested value'],
        ['___BR___ <br\t/>', '<br/> <br/>'],
    ];
    for (const [input, expected] of cases) assert.equal(sanitize(input), expected);
});

test('escapes generated tags and preserves quoted attributes', () => {
    for (let i = 0; i < 1000; i++) {
        const tag = `tag_${i}:part.test`;
        const input = `<${tag} value=${i}> </${tag}> <${tag} value="${i}">`;
        const expected = `&lt;${tag} value=${i}&gt; &lt;/${tag}&gt; <${tag} value="${i}">`;
        assert.equal(sanitize(input), expected);
    }
});

test('handles repeated unfinished tags and long whitespace', () => {
    const input = '<tag '.repeat(50000) + ' '.repeat(100000) + '"';
    assert.equal(sanitize(input), input.trim());
});
