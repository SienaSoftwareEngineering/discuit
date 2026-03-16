import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const rootPath = path.join(__dirname, '../');

fs.cpSync(path.join(rootPath, 'dist-sw'), path.join(rootPath, 'dist'), { recursive: true });

fs.rmSync(path.join(rootPath, 'dist-sw'), { recursive: true, force: true });