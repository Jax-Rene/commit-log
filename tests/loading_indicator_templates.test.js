const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

function read(relPath) {
        return fs.readFileSync(path.join(process.cwd(), relPath), 'utf8');
}

test('loading indicator CSS animation is defined', () => {
        const css = read('web/static/css/input.css');
        assert.match(css, /\.ui-loading\s*\{/);
        assert.match(css, /@keyframes\s+ui-loading-spin/);
});

test('public templates use unified ui-loading class for loading states', () => {
        const files = [
                'web/template/public/home.html',
                'web/template/public/gallery.html',
                'web/template/public/partials/post_cards.html',
                'web/template/public/partials/gallery_items.html',
                'web/template/layout/public_base.html',
        ];

        for (const file of files) {
                const content = read(file);
                assert.match(content, /ui-loading/, `${file} should include ui-loading class`);
        }
});

test('admin templates use unified ui-loading class for loading states', () => {
        const files = [
                'web/template/admin/gallery_manage.html',
                'web/template/admin/tag_manage.html',
                'web/template/components/contact_manager.html',
        ];

        for (const file of files) {
                const content = read(file);
                assert.match(content, /ui-loading/, `${file} should include ui-loading class`);
        }
});
