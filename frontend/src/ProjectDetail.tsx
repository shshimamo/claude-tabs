import { useState, useEffect, useRef } from 'react'
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

  useEffect(() => {
    setDraft(project)
    setDirty(false)
    setMemoEdit(false)
  }, [project.id])

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

  const save = () => {
    onUpdate(draft)
    setDirty(false)
  }

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
        <div className="project-memo-header">
          <span className="project-link-label">{t('memo', locale)}</span>
          <button className="project-link-add" onClick={() => setMemoEdit(!memoEdit)}>
            {memoEdit ? 'Preview' : 'Edit'}
          </button>
        </div>
        {memoEdit ? (
          <textarea
            className="project-memo-textarea"
            value={draft.memo}
            onChange={e => change('memo', e.target.value)}
            placeholder="Markdown..."
          />
        ) : (
          <div className="project-memo-preview">
            {draft.memo ? (
              <ReactMarkdown
                components={{
                  a: ({ href, children }) => (
                    <a href={href} target="_blank" rel="noopener noreferrer">{children}</a>
                  ),
                }}
              >{draft.memo}</ReactMarkdown>
            ) : (
              <span className="project-memo-empty">-</span>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
