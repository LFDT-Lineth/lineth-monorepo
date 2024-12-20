/**
 * Runs as git pre-commit hook
 * Filters the list of changed files on 'git commit'
 * If *.ts files in specified projects are detected, runs the 'lint:ts:fix' package.json script for that project
 * E.g. if a *.ts file is changed in /sdk, then this script will run 'pnpm run lint:ts:fix' in the /sdk project
 */

const fs = require('fs');
const { execSync } = require('child_process');

/**
 * ENUMS
 */

// File extensions to filter for
const FILE_EXTENSION = {
    TYPESCRIPT: "TYPESCRIPT",
    SOLIDITY: "SOLIDITY",
}

// Projects to filter for
const FOLDER = {
    BRIDGEUI: "BRIDGEUI",
    CONTRACTS: "CONTRACTS",
    E2E: "E2E",
    OPERATIONS: "OPERATIONS",
    POSTMAN: "POSTMAN",
    SDK: "SDK",
}

// Project runtimes
const RUNTIME = {
    NODEJS: "NODEJS"
}

/**
 * MAPPINGS
 */

// File extension => regex
const FILE_EXTENSION_FILTERS = {
    [FILE_EXTENSION.TYPESCRIPT]: "\.ts$",
    [FILE_EXTENSION.SOLIDITY]: "\.sol$",
};

// File extension => script in package.json to run
const FILE_EXTENSION_LINTING_COMMAND = {
    [FILE_EXTENSION.TYPESCRIPT]: "pnpm run lint:ts:fix",
    [FILE_EXTENSION.SOLIDITY]: "pnpm run lint:sol:fix",
};

// Project => Path in monorepo
const FOLDER_PATH = {
    [FOLDER.BRIDGEUI]: "bridge-ui/",
    [FOLDER.CONTRACTS]: "contracts/",
    [FOLDER.E2E]: "e2e/",
    [FOLDER.OPERATIONS]: "operations/",
    [FOLDER.POSTMAN]: "postman/",
    [FOLDER.SDK]: "sdk/",
};

// Project => List of changed files
const FOLDER_CHANGED_FILES = {
    [FOLDER.BRIDGEUI]: new Array(),
    [FOLDER.CONTRACTS]: new Array(),
    [FOLDER.E2E]: new Array(),
    [FOLDER.OPERATIONS]: new Array(),
    [FOLDER.POSTMAN]: new Array(),
    [FOLDER.SDK]: new Array(),
};

// Project => Runtime
const FOLDER_RUNTIME = {
    [FOLDER.BRIDGEUI]: RUNTIME.NODEJS,
    [FOLDER.CONTRACTS]: RUNTIME.NODEJS,
    [FOLDER.E2E]: RUNTIME.NODEJS,
    [FOLDER.OPERATIONS]: RUNTIME.NODEJS,
    [FOLDER.POSTMAN]: RUNTIME.NODEJS,
    [FOLDER.SDK]: RUNTIME.NODEJS,
};

/**
 * MAIN FUNCTION
 */

main();

function main() {
    const changedFileList = getChangedFileList();
    partitionChangedFileList(changedFileList);

    for (const folder in FOLDER) {
        if (!isDependenciesInstalled(folder)) {
            console.error(`Dependencies not installed in ${FOLDER_PATH[folder]}, exiting...`)
            process.exit(1);
        }
        const changedFileExtensions = getChangedFileExtensions(folder);
        executeLinting(folder, changedFileExtensions);
    }

    updateGitIndex();
}

/**
 * HELPER FUNCTIONS
 */

/**
 * Gets a list of changed files in the git commit
 * @returns {string[]}
 */
function getChangedFileList() {
    try {
        const cmd = 'git diff --name-only HEAD'
        const stdout = execSync(cmd, { encoding: 'utf8' });
        return stdout.split('\n').filter(file => file.trim() !== '');
    } catch (error) {
        console.error($`Error running ${cmd}:`, error.message);
        process.exit(1)
    }
}

/**
 * Partitions list of changed files from getChangedFileList() by project
 * Stores results in FOLDER_CHANGED_FILES
 * @param {string[]}
 */
function partitionChangedFileList(_changedFileList) {
    for (const file of _changedFileList) {
        for (const path in FOLDER) {
            if (file.match(new RegExp(`^${FOLDER_PATH[path]}`))) {
                FOLDER_CHANGED_FILES[path].push(file);
            }
        }
    }
}

/**
 * Checks if runtime dependencies are installed for a project
 * @param {FOLDER}
 * @returns {boolean}
 */
function isDependenciesInstalled(_folder) {
    const runtime = FOLDER_RUNTIME[_folder];
    const path = FOLDER_PATH[_folder];

    switch(runtime) {
        case RUNTIME.NODEJS:
            const dependencyFolder = `${path}node_modules`
            return fs.existsSync(dependencyFolder)
        default:
            console.error(`${runtime} runtime not supported.`);
            return false
    }
}

/**
 * Resolve list of changed file extensions for a project
 * @param {FOLDER}
 * @returns {FILE_EXTENSION[]}
 */
function getChangedFileExtensions(_folder) {
    // Use sets to implement early exit from loop, once we have matched all configured file extensions
    const remainingFileExtensionsSet = new Set(Object.values(FILE_EXTENSION));
    const foundFileExtensionsSet = new Set();

    for (const file of FOLDER_CHANGED_FILES[_folder]) {
        for (const fileExtension of remainingFileExtensionsSet) {
            if (file.match(new RegExp(FILE_EXTENSION_FILTERS[fileExtension]))) {
                foundFileExtensionsSet.add(fileExtension);
                remainingFileExtensionsSet.delete(fileExtension);
            }
        }

        // No more remaining file extensions to look for
        if (remainingFileExtensionsSet.size == 0) break; 
    }

    return Array.from(foundFileExtensionsSet);
}

/**
 * Execute linting command
 * @param {FOLDER, FILE_EXTENSION[]}
 */
function executeLinting(_folder, _changedFileExtensions) {
    for (const fileExtension of _changedFileExtensions) {
        const path = FOLDER_PATH[_folder];
        const cmd = FILE_EXTENSION_LINTING_COMMAND[fileExtension];
        console.log(`${fileExtension} change found in ${path}, linting...`);
        try {
            // Execute command synchronously and stream output directly to the current stdout
            execSync(`
                cd ${path};
                ${cmd};
            `, { stdio: 'inherit' });
        } catch (error) {
            console.error(`Error:`, error.message);
            console.error(`Exiting...`);
            process.exit(1);
        }
    }
}

/**
 * Redo `git add` for files updated during executeLinting(), so that they are not left out of the commit
 * The difference between 'git add .' and 'git update-index --again', is that the latter will not include untracked files
 */
function updateGitIndex() {
    try {
        const cmd = 'git update-index --again'
        execSync(cmd, { stdio: 'inherit' });
    } catch (error) {
        console.error($`Error running ${cmd}:`, error.message);
        process.exit(1);
    }
}