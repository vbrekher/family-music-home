// JSONForms 2 uses the retired uuid/v1 entrypoint. Keep that import compatible
// while using the maintained, patched UUID dependency instead of UUID 3/8.
export { v1 as default } from 'uuid'
