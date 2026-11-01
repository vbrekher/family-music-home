import React, { useRef } from 'react'
import DOMPurify from 'dompurify'
import { Button, Typography, makeStyles } from '@material-ui/core'
export const safeMessage = (value) =>
  DOMPurify.sanitize(value || '', {
    ALLOWED_TAGS: [
      'p',
      'br',
      'strong',
      'b',
      'em',
      'i',
      'ul',
      'ol',
      'li',
      'blockquote',
    ],
    ALLOWED_ATTR: [],
  })
const useStyles = makeStyles((theme) => ({
  editor: {
    border: `1px solid ${theme.palette.text.secondary}`,
    borderRadius: 4,
    minHeight: 120,
    padding: 12,
    marginBottom: 16,
    overflowWrap: 'anywhere',
    '&:focus': { outline: `2px solid ${theme.palette.primary.main}` },
  },
}))
export default function RichMessage({ value, onChange }) {
  const editor = useRef(null),
    initial = useRef(safeMessage(value)),
    c = useStyles()
  const report = () => onChange(safeMessage(editor.current.innerHTML))
  const format = (command) => {
    editor.current.focus()
    document.execCommand(command, false)
    report()
  }
  return (
    <section aria-label="Compose message">
      <Typography id="family-message-label">Message</Typography>
      <div role="toolbar" aria-label="Message formatting">
        {[
          ['bold', 'Bold'],
          ['italic', 'Italic'],
          ['insertUnorderedList', 'Bullet list'],
          ['removeFormat', 'Clear formatting'],
        ].map(([command, label]) => (
          <Button
            key={command}
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => format(command)}
          >
            {label}
          </Button>
        ))}
      </div>
      <div
        ref={editor}
        contentEditable
        suppressContentEditableWarning
        role="textbox"
        aria-multiline="true"
        aria-labelledby="family-message-label"
        className={c.editor}
        dangerouslySetInnerHTML={{ __html: initial.current }}
        onInput={report}
        onDrop={(e) => e.preventDefault()}
        onPaste={(e) => {
          e.preventDefault()
          document.execCommand(
            'insertText',
            false,
            e.clipboardData.getData('text/plain').slice(0, 16000),
          )
          report()
        }}
      />
      {value.length > 16000 && (
        <Typography role="alert">
          Shorten the message before saving (maximum 16,000 characters including
          formatting).
        </Typography>
      )}
    </section>
  )
}
