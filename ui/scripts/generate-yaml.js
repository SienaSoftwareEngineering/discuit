import { exec } from 'child_process';
import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
// Get the root path
//const rootPath = path.join(import.meta.dirname, '../../');
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootPath = path.join(__dirname, '../../');

// Run the command to get the config
(async () => {
  console.log("in async, before exec")
  await exec('./discuit inject-config', { cwd: '../' }, async (error, stdout, stderr) => {
    if (error) {
      console.error(`exec error: ${error}`);
      return;
    }
    if (stderr) {
      console.error(`stderr: ${stderr}`);
      return;
    }
console.log("in async, before exec", rootPath)
    // Make ui-config.yaml
    await fs.writeFile(path.join(rootPath, 'ui-config.yaml'), stdout, (err) => {
      if (err) {
        console.error(err);
      } else {
        console.log('ui-config.yaml has been written');
      }
    });
  });
})();
