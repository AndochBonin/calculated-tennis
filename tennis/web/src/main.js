import { mount } from 'svelte';
import App from './App.svelte';
import '@fontsource/inter-tight/400.css';
import '@fontsource/inter-tight/500.css';
import '@fontsource/inter-tight/600.css';
import './app.css';

mount(App, { target: document.getElementById('app') });
