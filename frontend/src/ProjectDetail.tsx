import { useState, useEffect, useRef, useCallback } from 'react'
import ReactMarkdown from 'react-markdown'
import { t, type Locale } from './i18n'

type NamedLink = { name: string; url: string }
type LinkSection = { label: string; links: NamedLink[] }

export const DEFAULT_LINK_SECTIONS: LinkSection[] = [
  { label: 'PRD', links: [] },
  { label: 'Spec', links: [] },
  { label: 'NotebookLM', links: [] },
  { label: 'Slack', links: [] },
]

export type Project = {
  id: string
  name: string
  link_sections: LinkSection[]
  memo: string
  archived: boolean
  created_at: string
  order: number
}

type Props = {
  project: Project
  onUpdate: (project: Project) => void
  locale: Locale
}

function LinkSectionView({ section, onChange, onRemoveSection, onRenameSection }: {
  section: LinkSection
  onChange: (links: NamedLink[]) => void
  onRemoveSection: () => void
  onRenameSection: (name: string) => void
}) {
  const [editing, setEditing] = useState(false)
  const [editLabel, setEditLabel] = useState(section.label)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => { if (editing) inputRef.current?.focus() }, [editing])

  const update = (idx: number, field: 'name' | 'url', value: string) => {
    const next = section.links.map((l, i) => i === idx ? { ...l, [field]: value } : l)
    onChange(next)
  }
  const remove = (idx: number) => onChange(section.links.filter((_, i) => i !== idx))
  const add = () => onChange([...section.links, { name: '', url: '' }])

  return (
    <div className="project-link-section">
      <div className="project-link-header">
        {editing ? (
          <input
            ref={inputRef}
            className="group-name-input"
            value={editLabel}
            onChange={e => setEditLabel(e.target.value)}
            onBlur={() => { setEditing(false); if (editLabel.trim()) onRenameSection(editLabel.trim()) }}
            onKeyDown={e => { if (e.key === 'Enter') (e.target as HTMLInputElement).blur(); if (e.key === 'Escape') setEditing(false) }}
          />
        ) : (
          <span className="project-link-label" onDoubleClick={() => { setEditing(true); setEditLabel(section.label) }}>
            {section.label}
          </span>
        )}
        <button className="project-link-add" onClick={add}>+</button>
        <button className="project-link-remove" onClick={onRemoveSection} title="Remove section">x</button>
      </div>
      {section.links.map((link, idx) => (
        <div key={idx} className="project-link-row">
          <input
            className="project-link-name"
            value={link.name}
            onChange={e => update(idx, 'name', e.target.value)}
            placeholder="Name"
          />
          <input
            className="project-link-url"
            value={link.url}
            onChange={e => update(idx, 'url', e.target.value)}
            placeholder="https://..."
          />
          {link.url && (
            <a href={link.url} target="_blank" rel="noopener noreferrer" className="project-link-open">
              &#8599;
            </a>
          )}
          <button className="project-link-remove" onClick={() => remove(idx)}>x</button>
        </div>
      ))}
      {section.links.length > 0 && section.links.some(l => l.url) && (
        <div className="project-link-list">
          {section.links.filter(l => l.url).map((link, idx) => (
            <a key={idx} href={link.url} target="_blank" rel="noopener noreferrer" className="project-link-chip">
              {link.name || link.url}
            </a>
          ))}
        </div>
      )}
    </div>
  )
}

export default function ProjectDetail({ project, onUpdate, locale }: Props) {
  const [draft, setDraft] = useState(project)
  const [dirty, setDirty] = useState(false)
  const [memoEdit, setMemoEdit] = useState(false)
  const draftRef = useRef(draft)
  const dirtyRef = useRef(dirty)
  const memoRef = useRef<HTMLTextAreaElement>(null)

  useEffect(() => { draftRef.current = draft }, [draft])
  useEffect(() => { dirtyRef.current = dirty }, [dirty])

  useEffect(() => {
    setDraft(project)
    setDirty(false)
    setMemoEdit(false)
  }, [project.id])

  const save = useCallback(() => {
    if (dirtyRef.current) {
      onUpdate(draftRef.current)
      setDirty(false)
    }
  }, [onUpdate])

  // Cmd+S to save
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 's') {
        e.preventDefault()
        save()
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [save])

  const change = <K extends keyof Project>(key: K, value: Project[K]) => {
    setDraft(prev => ({ ...prev, [key]: value }))
    setDirty(true)
  }

  const updateSection = (idx: number, links: NamedLink[]) => {
    const sections = draft.link_sections.map((s, i) => i === idx ? { ...s, links } : s)
    change('link_sections', sections)
  }

  const renameSection = (idx: number, label: string) => {
    const sections = draft.link_sections.map((s, i) => i === idx ? { ...s, label } : s)
    change('link_sections', sections)
  }

  const removeSection = (idx: number) => {
    change('link_sections', draft.link_sections.filter((_, i) => i !== idx))
  }

  const addSection = () => {
    change('link_sections', [...draft.link_sections, { label: 'New', links: [] }])
  }

  // Memo: click preview to enter edit, blur textarea to exit and save
  const handleMemoBlur = useCallback((e: React.FocusEvent) => {
    // Check if focus moved outside the memo area
    const memoContainer = e.currentTarget.closest('.project-memo')
    if (memoContainer && e.relatedTarget && memoContainer.contains(e.relatedTarget as Node)) return
    setMemoEdit(false)
    // Save on blur using latest draft via ref
    setTimeout(() => {
      if (dirtyRef.current) {
        onUpdate(draftRef.current)
        setDirty(false)
      }
    }, 0)
  }, [onUpdate])

  useEffect(() => {
    if (memoEdit && memoRef.current) memoRef.current.focus()
  }, [memoEdit])

  return (
    <div className="project-detail">
      <div className="project-detail-header">
        <input
          className="project-name-input"
          value={draft.name}
          onChange={e => change('name', e.target.value)}
          placeholder="Project name"
        />
        {dirty && (
          <button className="action-btn" onClick={save}>{t('save', locale)}</button>
        )}
      </div>

      <div className="project-links">
        {draft.link_sections.map((section, idx) => (
          <LinkSectionView
            key={idx}
            section={section}
            onChange={links => updateSection(idx, links)}
            onRenameSection={label => renameSection(idx, label)}
            onRemoveSection={() => removeSection(idx)}
          />
        ))}
        <button className="project-add-section" onClick={addSection}>
          + {t('add_link', locale)}
        </button>
      </div>

      <div className="project-memo">
        <span className="project-link-label">{t('memo', locale)}</span>
        {memoEdit ? (
          <textarea
            ref={memoRef}
            className="project-memo-textarea"
            value={draft.memo}
            onChange={e => change('memo', e.target.value)}
            onBlur={handleMemoBlur}
            placeholder="Markdown..."
          />
        ) : (
          <div className="project-memo-preview" onClick={() => setMemoEdit(true)}>
            {draft.memo ? (
              <ReactMarkdown
                components={{
                  a: ({ href, children }) => (
                    <a href={href} target="_blank" rel="noopener noreferrer" onClick={e => e.stopPropagation()}>{children}</a>
                  ),
                }}
              >{draft.memo}</ReactMarkdown>
            ) : (
              <span className="project-memo-empty">Click to edit...</span>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
