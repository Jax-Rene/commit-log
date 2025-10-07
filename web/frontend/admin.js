import Alpine from 'alpinejs';
import htmx from 'htmx.org';
import flatpickr from 'flatpickr';
import { Mandarin } from 'flatpickr/dist/l10n/zh.js';
import EasyMDE from 'easymde';
import Cropper from 'cropperjs';
import hljs from 'highlight.js/lib/common';

import 'flatpickr/dist/flatpickr.min.css';
import 'flatpickr/dist/themes/airbnb.css';
import 'easymde/dist/easymde.min.css';
import 'cropperjs/dist/cropper.min.css';
import 'highlight.js/styles/github.css';
import '../static/css/editor.css';
import '../static/css/input.css';

globalThis.htmx = htmx;
globalThis.Alpine = Alpine;
globalThis.flatpickr = flatpickr;
globalThis.EasyMDE = EasyMDE;
globalThis.Cropper = Cropper;
globalThis.hljs = hljs;

flatpickr.localize(Mandarin);

Alpine.start();
