import Alpine from 'alpinejs';
import htmx from 'htmx.org';

import '../static/css/input.css';

globalThis.htmx = htmx;
globalThis.Alpine = Alpine;

Alpine.start();
