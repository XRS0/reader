import {
  cloneElement,
  createContext,
  forwardRef,
  isValidElement,
  useCallback,
  useContext,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type ButtonHTMLAttributes,
  type CSSProperties,
  type HTMLAttributes,
  type InputHTMLAttributes,
  type ReactElement,
  type ReactNode,
  type SelectHTMLAttributes,
  type TextareaHTMLAttributes
} from 'react'
import { createPortal } from 'react-dom'
import {
  AlertCircle,
  Check,
  ChevronRight,
  CircleAlert,
  LoaderCircle,
  Search,
  X,
  type LucideIcon
} from 'lucide-react'
import clsx from 'clsx'
import { useTranslation } from 'react-i18next'
import styles from './ui.module.css'

export type ButtonVariant = 'primary' | 'accent' | 'secondary' | 'ghost' | 'danger'
export type ButtonSize = 'small' | 'medium' | 'large'

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
  loading?: boolean
  startIcon?: LucideIcon
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  {
    variant = 'secondary',
    size = 'medium',
    loading = false,
    startIcon: StartIcon,
    disabled,
    className,
    children,
    type = 'button',
    ...props
  },
  ref
) {
  return (
    <button
      ref={ref}
      type={type}
      className={clsx(
        styles.button,
        styles[`button${variant[0]!.toUpperCase()}${variant.slice(1)}`],
        size !== 'medium' && styles[`button${size[0]!.toUpperCase()}${size.slice(1)}`],
        className
      )}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      {...props}
    >
      {loading ? (
        <span className={styles.spinner} aria-hidden="true" />
      ) : StartIcon ? (
        <StartIcon size={16} aria-hidden="true" />
      ) : null}
      {children}
    </button>
  )
})

export interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  label: string
  icon: LucideIcon
  size?: 'small' | 'medium'
  loading?: boolean
}

export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(function IconButton(
  {
    label,
    icon: Icon,
    size = 'medium',
    loading = false,
    className,
    disabled,
    type = 'button',
    ...props
  },
  ref
) {
  return (
    <button
      ref={ref}
      type={type}
      className={clsx(styles.iconButton, size === 'small' && styles.iconButtonSmall, className)}
      aria-label={label}
      title={label}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      {...props}
    >
      {loading ? (
        <span className={styles.spinner} aria-hidden="true" />
      ) : (
        <Icon size={size === 'small' ? 16 : 18} aria-hidden="true" />
      )}
    </button>
  )
})

export interface FieldProps {
  label: string
  htmlFor?: string
  hint?: string
  error?: string
  children: ReactNode
  className?: string
}

export function Field({ label, htmlFor, hint, error, children, className }: FieldProps) {
  return (
    <div className={clsx(styles.field, className)}>
      <label className={styles.label} htmlFor={htmlFor}>
        {label}
      </label>
      {children}
      {hint && !error ? <span className={styles.hint}>{hint}</span> : null}
      {error ? (
        <span className={styles.errorText} role="alert">
          {error}
        </span>
      ) : null}
    </div>
  )
}

export const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(
  function Input({ className, ...props }, ref) {
    return <input ref={ref} className={clsx(styles.input, className)} {...props} />
  }
)

export const Textarea = forwardRef<
  HTMLTextAreaElement,
  TextareaHTMLAttributes<HTMLTextAreaElement>
>(function Textarea({ className, ...props }, ref) {
  return <textarea ref={ref} className={clsx(styles.textarea, className)} {...props} />
})

export const Select = forwardRef<HTMLSelectElement, SelectHTMLAttributes<HTMLSelectElement>>(
  function Select({ className, children, ...props }, ref) {
    return (
      <select ref={ref} className={clsx(styles.select, className)} {...props}>
        {children}
      </select>
    )
  }
)

export interface SearchInputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  label: string
  onClear?: () => void
}

export const SearchInput = forwardRef<HTMLInputElement, SearchInputProps>(function SearchInput(
  { label, value, onClear, className, ...props },
  ref
) {
  const hasValue = typeof value === 'string' && value.length > 0
  return (
    <div className={clsx(styles.searchWrap, className)}>
      <Search className={styles.searchIcon} size={16} aria-hidden="true" />
      <Input
        ref={ref}
        type="search"
        value={value}
        aria-label={label}
        className={styles.searchInput}
        {...props}
      />
      {hasValue && onClear ? (
        <IconButton
          className={styles.searchClear}
          size="small"
          label={label}
          icon={X}
          onClick={onClear}
        />
      ) : null}
    </div>
  )
})

export interface ComboboxOption {
  value: string
  label: string
}

export function Combobox({
  id,
  label,
  options,
  value,
  onChange,
  placeholder
}: {
  id: string
  label: string
  options: ComboboxOption[]
  value: string
  onChange: (value: string) => void
  placeholder?: string
}) {
  const listId = `${id}-options`
  return (
    <>
      <Input
        id={id}
        role="combobox"
        aria-label={label}
        aria-autocomplete="list"
        aria-controls={listId}
        list={listId}
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
      />
      <datalist id={listId}>
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </datalist>
    </>
  )
}

export function Checkbox({
  label,
  checked,
  onChange,
  disabled
}: {
  label: string
  checked: boolean
  onChange: (checked: boolean) => void
  disabled?: boolean
}) {
  return (
    <label className={styles.checkboxRow}>
      <input
        className={styles.checkbox}
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={(event) => onChange(event.target.checked)}
      />
      <span>{label}</span>
    </label>
  )
}

export function Switch({
  label,
  checked,
  onChange,
  disabled
}: {
  label: string
  checked: boolean
  onChange: (checked: boolean) => void
  disabled?: boolean
}) {
  return (
    <label className={styles.switchRow}>
      <button
        type="button"
        role="switch"
        className={styles.switch}
        aria-checked={checked}
        aria-label={label}
        disabled={disabled}
        onClick={() => onChange(!checked)}
      />
      <span>{label}</span>
    </label>
  )
}

export function Badge({
  children,
  tone = 'neutral',
  className
}: {
  children: ReactNode
  tone?: 'neutral' | 'accent' | 'danger'
  className?: string
}) {
  return (
    <span
      className={clsx(
        styles.badge,
        tone === 'accent' && styles.badgeAccent,
        tone === 'danger' && styles.badgeDanger,
        className
      )}
    >
      {children}
    </span>
  )
}

export function Avatar({ name, src }: { name: string; src?: string }) {
  const initials = name
    .trim()
    .split(/\s+/)
    .slice(0, 2)
    .map((item) => item[0])
    .join('')
    .toUpperCase()
  return (
    <span className={styles.avatar} aria-label={name}>
      {src ? <img alt="" src={src} /> : initials || '?'}
    </span>
  )
}

export function ProgressBar({ value, label }: { value: number; label: string }) {
  const bounded = Math.max(0, Math.min(100, value))
  return (
    <div
      className={styles.progressTrack}
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={Math.round(bounded)}
    >
      <div className={styles.progressValue} style={{ width: `${bounded}%` }} />
    </div>
  )
}

function useFocusTrap(open: boolean, onClose: () => void) {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!open) return undefined
    const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null
    const root = ref.current
    const focusable = root?.querySelectorAll<HTMLElement>(
      'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
    )
    focusable?.[0]?.focus()
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        onClose()
        return
      }
      if (event.key !== 'Tab' || !focusable?.length) return
      const first = focusable[0]!
      const last = focusable[focusable.length - 1]!
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault()
        first.focus()
      }
    }
    document.addEventListener('keydown', onKeyDown)
    document.body.style.overflow = 'hidden'
    return () => {
      document.removeEventListener('keydown', onKeyDown)
      document.body.style.overflow = ''
      previous?.focus()
    }
  }, [open, onClose])
  return ref
}

function Portal({ children }: { children: ReactNode }) {
  const target = document.getElementById('portal-root') ?? document.body
  return createPortal(children, target)
}

export interface DialogProps {
  open: boolean
  onClose: () => void
  title: string
  description?: string
  children: ReactNode
  footer?: ReactNode
  className?: string
  closeLabel?: string
}

export function Dialog({
  open,
  onClose,
  title,
  description,
  children,
  footer,
  className,
  closeLabel = 'Close'
}: DialogProps) {
  const titleId = useId()
  const descriptionId = useId()
  const ref = useFocusTrap(open, onClose)
  if (!open) return null
  return (
    <Portal>
      <div
        className={styles.overlay}
        onMouseDown={(event) => event.target === event.currentTarget && onClose()}
      >
        <div
          ref={ref}
          className={clsx(styles.dialog, className)}
          role="dialog"
          aria-modal="true"
          aria-labelledby={titleId}
          aria-describedby={description ? descriptionId : undefined}
        >
          <div className={styles.dialogHeader}>
            <div>
              <h2 id={titleId} className={styles.dialogTitle}>
                {title}
              </h2>
              {description ? (
                <p id={descriptionId} className={styles.dialogDescription}>
                  {description}
                </p>
              ) : null}
            </div>
            <IconButton size="small" icon={X} label={closeLabel} onClick={onClose} />
          </div>
          <div className={styles.dialogBody}>{children}</div>
          {footer ? <div className={styles.dialogFooter}>{footer}</div> : null}
        </div>
      </div>
    </Portal>
  )
}

export function AlertDialog({
  open,
  onClose,
  onConfirm,
  title,
  description,
  confirmLabel,
  cancelLabel,
  confirmLoading = false
}: {
  open: boolean
  onClose: () => void
  onConfirm: () => void
  title: string
  description: string
  confirmLabel: string
  cancelLabel: string
  confirmLoading?: boolean
}) {
  return (
    <Dialog
      open={open}
      onClose={onClose}
      title={title}
      description={description}
      closeLabel={cancelLabel}
      footer={
        <>
          <Button onClick={onClose}>{cancelLabel}</Button>
          <Button variant="danger" loading={confirmLoading} onClick={onConfirm}>
            {confirmLabel}
          </Button>
        </>
      }
    >
      <span />
    </Dialog>
  )
}

export function Drawer({
  open,
  onClose,
  label,
  children
}: {
  open: boolean
  onClose: () => void
  label: string
  children: ReactNode
}) {
  const ref = useFocusTrap(open, onClose)
  if (!open) return null
  return (
    <Portal>
      <div
        className={clsx(styles.overlay, styles.drawerOverlay)}
        onMouseDown={(event) => event.target === event.currentTarget && onClose()}
      >
        <div ref={ref} className={styles.drawer} role="dialog" aria-modal="true" aria-label={label}>
          {children}
        </div>
      </div>
    </Portal>
  )
}

export function BottomSheet({
  open,
  onClose,
  label,
  children
}: {
  open: boolean
  onClose: () => void
  label: string
  children: ReactNode
}) {
  const ref = useFocusTrap(open, onClose)
  if (!open) return null
  return (
    <Portal>
      <div
        className={clsx(styles.overlay, styles.bottomOverlay)}
        onMouseDown={(event) => event.target === event.currentTarget && onClose()}
      >
        <div
          ref={ref}
          className={styles.bottomSheet}
          role="dialog"
          aria-modal="true"
          aria-label={label}
        >
          {children}
        </div>
      </div>
    </Portal>
  )
}

export function Popover({
  open,
  position,
  children,
  className
}: {
  open: boolean
  position?: CSSProperties
  children: ReactNode
  className?: string
}) {
  if (!open) return null
  return (
    <div className={clsx(styles.popover, className)} style={position}>
      {children}
    </div>
  )
}

export interface MenuItem {
  id: string
  label: string
  icon?: LucideIcon
  danger?: boolean
  disabled?: boolean
  separatorBefore?: boolean
  onSelect: () => void
}

export function Menu({
  items,
  label,
  onClose
}: {
  items: MenuItem[]
  label: string
  onClose?: () => void
}) {
  const [activeIndex, setActiveIndex] = useState(0)
  const refs = useRef<Array<HTMLButtonElement | null>>([])
  const enabled = items.filter((item) => !item.disabled)
  const select = (item: MenuItem) => {
    if (item.disabled) return
    item.onSelect()
    onClose?.()
  }
  const onKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Escape') {
      event.preventDefault()
      onClose?.()
      return
    }
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      const delta = event.key === 'ArrowDown' ? 1 : -1
      const next = (activeIndex + delta + enabled.length) % enabled.length
      setActiveIndex(next)
      const originalIndex = items.indexOf(enabled[next]!)
      refs.current[originalIndex]?.focus()
    }
  }
  return (
    <div className={styles.menu} role="menu" aria-label={label} onKeyDown={onKeyDown}>
      {items.map((item, index) => {
        const itemEnabledIndex =
          items.slice(0, index + 1).filter((value) => !value.disabled).length - 1
        const Icon = item.icon
        return (
          <div key={item.id}>
            {item.separatorBefore ? (
              <div className={styles.menuSeparator} role="separator" />
            ) : null}
            <button
              ref={(node) => {
                refs.current[index] = node
              }}
              type="button"
              role="menuitem"
              tabIndex={item.disabled ? -1 : itemEnabledIndex === activeIndex ? 0 : -1}
              disabled={item.disabled}
              className={clsx(styles.menuItem, item.danger && styles.menuItemDanger)}
              onFocus={() => setActiveIndex(itemEnabledIndex)}
              onClick={() => select(item)}
            >
              {Icon ? <Icon size={16} aria-hidden="true" /> : null}
              {item.label}
            </button>
          </div>
        )
      })}
    </div>
  )
}

export function DropdownMenu({
  trigger,
  items,
  label,
  align = 'right'
}: {
  trigger: ReactElement
  items: MenuItem[]
  label: string
  align?: 'left' | 'right'
}) {
  const [open, setOpen] = useState(false)
  const rootRef = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!open) return undefined
    const close = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', close)
    return () => document.removeEventListener('mousedown', close)
  }, [open])
  const typedTrigger = trigger as ReactElement<ButtonHTMLAttributes<HTMLButtonElement>>
  const triggerNode = isValidElement(typedTrigger)
    ? cloneElement(typedTrigger, {
        'aria-haspopup': 'menu',
        'aria-expanded': open,
        onClick: (event) => {
          typedTrigger.props.onClick?.(event)
          setOpen((value) => !value)
        }
      })
    : trigger
  return (
    <div ref={rootRef} style={{ position: 'relative', display: 'inline-flex' }}>
      {triggerNode}
      {open ? (
        <div className={styles.popover} style={{ top: 'calc(100% + 4px)', [align]: 0 }}>
          <Menu items={items} label={label} onClose={() => setOpen(false)} />
        </div>
      ) : null}
    </div>
  )
}

export function ContextMenu({
  children,
  items,
  label
}: {
  children: ReactElement
  items: MenuItem[]
  label: string
}) {
  const [position, setPosition] = useState<{ x: number; y: number } | null>(null)
  useEffect(() => {
    if (!position) return undefined
    const close = () => setPosition(null)
    document.addEventListener('mousedown', close)
    document.addEventListener('scroll', close, true)
    return () => {
      document.removeEventListener('mousedown', close)
      document.removeEventListener('scroll', close, true)
    }
  }, [position])
  const typedChild = children as ReactElement<HTMLAttributes<HTMLElement>>
  const child = cloneElement(typedChild, {
    onContextMenu: (event) => {
      typedChild.props.onContextMenu?.(event)
      event.preventDefault()
      setPosition({ x: event.clientX, y: event.clientY })
    }
  })
  return (
    <>
      {child}
      {position ? (
        <div
          className={styles.popover}
          style={{ position: 'fixed', left: position.x, top: position.y }}
          onMouseDown={(event) => event.stopPropagation()}
        >
          <Menu items={items} label={label} onClose={() => setPosition(null)} />
        </div>
      ) : null}
    </>
  )
}

export function Tooltip({ content, children }: { content: string; children: ReactElement }) {
  const [position, setPosition] = useState<{ top: number; left: number } | null>(null)
  const show = (element: HTMLElement) => {
    const rect = element.getBoundingClientRect()
    setPosition({ top: rect.top + rect.height / 2, left: rect.right + 8 })
  }
  const child = cloneElement(children as ReactElement<HTMLAttributes<HTMLElement>>, {
    onMouseEnter: (event) => show(event.currentTarget),
    onMouseLeave: () => setPosition(null),
    onFocus: (event) => show(event.currentTarget),
    onBlur: () => setPosition(null),
    'aria-describedby': position ? `tooltip-${content.replace(/\s/g, '-')}` : undefined
  })
  return (
    <>
      {child}
      {position ? (
        <Portal>
          <span
            id={`tooltip-${content.replace(/\s/g, '-')}`}
            role="tooltip"
            className={styles.tooltip}
            style={{ top: position.top, left: position.left, transform: 'translateY(-50%)' }}
          >
            {content}
          </span>
        </Portal>
      ) : null}
    </>
  )
}

export interface TabItem {
  id: string
  label: string
  content: ReactNode
}

export function Tabs({ items, defaultTab }: { items: TabItem[]; defaultTab?: string }) {
  const [active, setActive] = useState(defaultTab ?? items[0]?.id)
  const activeIndex = Math.max(
    0,
    items.findIndex((item) => item.id === active)
  )
  const onKeyDown = (event: React.KeyboardEvent) => {
    if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
    event.preventDefault()
    let next = activeIndex
    if (event.key === 'ArrowLeft') next = (activeIndex - 1 + items.length) % items.length
    if (event.key === 'ArrowRight') next = (activeIndex + 1) % items.length
    if (event.key === 'Home') next = 0
    if (event.key === 'End') next = items.length - 1
    const nextItem = items[next]
    if (!nextItem) return
    setActive(nextItem.id)
    event.currentTarget.querySelectorAll<HTMLButtonElement>('[role="tab"]')[next]?.focus()
  }
  const selected = items[activeIndex]
  return (
    <div>
      <div className={styles.tabsList} role="tablist" onKeyDown={onKeyDown}>
        {items.map((item) => (
          <button
            key={item.id}
            id={`tab-${item.id}`}
            type="button"
            role="tab"
            className={styles.tab}
            aria-selected={item.id === active}
            aria-controls={`panel-${item.id}`}
            tabIndex={item.id === active ? 0 : -1}
            onClick={() => setActive(item.id)}
          >
            {item.label}
          </button>
        ))}
      </div>
      {selected ? (
        <div
          id={`panel-${selected.id}`}
          className={styles.tabPanel}
          role="tabpanel"
          aria-labelledby={`tab-${selected.id}`}
        >
          {selected.content}
        </div>
      ) : null}
    </div>
  )
}

export function EmptyState({
  icon: Icon,
  title,
  body,
  action
}: {
  icon?: LucideIcon
  title: string
  body: string
  action?: ReactNode
}) {
  return (
    <div className={styles.empty}>
      {Icon ? <Icon className={styles.emptyIcon} size={28} aria-hidden="true" /> : null}
      <h2 className={styles.emptyTitle}>{title}</h2>
      <p className={styles.emptyBody}>{body}</p>
      {action}
    </div>
  )
}

export function ErrorState({
  title,
  body,
  onRetry,
  retryLabel
}: {
  title: string
  body: string
  onRetry?: () => void
  retryLabel?: string
}) {
  return (
    <div className={styles.errorState} role="alert">
      <CircleAlert className={styles.errorIcon} size={28} aria-hidden="true" />
      <h2 className={styles.errorTitle}>{title}</h2>
      <p className={styles.errorBody}>{body}</p>
      {onRetry ? <Button onClick={onRetry}>{retryLabel ?? 'Retry'}</Button> : null}
    </div>
  )
}

export function LoadingState({ label }: { label: string }) {
  return (
    <div className={styles.loadingState} role="status">
      <LoaderCircle className={styles.spinner} size={22} aria-hidden="true" />
      <span>{label}</span>
    </div>
  )
}

export function Skeleton({
  width = '100%',
  height = 16,
  className
}: {
  width?: string | number
  height?: number
  className?: string
}) {
  return (
    <div
      className={clsx(styles.skeleton, className)}
      style={{ width, height }}
      aria-hidden="true"
    />
  )
}

export interface Column<T> {
  key: string
  header: ReactNode
  render: (item: T) => ReactNode
  className?: string
}

export function DataTable<T>({
  columns,
  items,
  rowKey,
  label,
  onRowClick
}: {
  columns: Column<T>[]
  items: T[]
  rowKey: (item: T) => string
  label: string
  onRowClick?: (item: T) => void
}) {
  return (
    <div className={styles.dataTableWrap}>
      <table className={styles.dataTable} aria-label={label}>
        <thead>
          <tr>
            {columns.map((column) => (
              <th key={column.key} className={column.className} scope="col">
                {column.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {items.map((item) => (
            <tr
              key={rowKey(item)}
              tabIndex={onRowClick ? 0 : undefined}
              onClick={() => onRowClick?.(item)}
              onKeyDown={(event) => {
                if (onRowClick && (event.key === 'Enter' || event.key === ' ')) {
                  event.preventDefault()
                  onRowClick(item)
                }
              }}
            >
              {columns.map((column) => (
                <td key={column.key} className={column.className}>
                  {column.render(item)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

export function Breadcrumbs({ items }: { items: Array<{ label: string; href?: string }> }) {
  return (
    <nav className={styles.breadcrumbs} aria-label="Breadcrumb">
      {items.map((item, index) => (
        <span key={`${item.label}-${index}`} style={{ display: 'contents' }}>
          {index ? <ChevronRight size={14} aria-hidden="true" /> : null}
          {item.href ? (
            <a className={styles.breadcrumbLink} href={item.href}>
              {item.label}
            </a>
          ) : (
            <span className={styles.breadcrumbCurrent} aria-current="page">
              {item.label}
            </span>
          )}
        </span>
      ))}
    </nav>
  )
}

interface ToastItem {
  id: string
  message: string
  tone: 'success' | 'error' | 'neutral'
}
interface ToastContextValue {
  notify: (message: string, tone?: ToastItem['tone']) => void
}
const ToastContext = createContext<ToastContextValue | null>(null)

export function ToastProvider({ children }: { children: ReactNode }) {
  const { t } = useTranslation()
  const [items, setItems] = useState<ToastItem[]>([])
  const notify = useCallback((message: string, tone: ToastItem['tone'] = 'neutral') => {
    const id = crypto.randomUUID()
    setItems((current) => [...current, { id, message, tone }])
    window.setTimeout(() => setItems((current) => current.filter((item) => item.id !== id)), 4000)
  }, [])
  const value = useMemo(() => ({ notify }), [notify])
  return (
    <ToastContext.Provider value={value}>
      {children}
      <div
        className={styles.toastRegion}
        role="region"
        aria-label="Notifications"
        aria-live="polite"
      >
        {items.map((item) => (
          <div key={item.id} className={styles.toast}>
            {item.tone === 'success' ? (
              <Check size={17} aria-hidden="true" />
            ) : item.tone === 'error' ? (
              <AlertCircle size={17} aria-hidden="true" />
            ) : null}
            <p className={styles.toastMessage}>{item.message}</p>
            <IconButton
              size="small"
              icon={X}
              label={t('common.close')}
              onClick={() => setItems((current) => current.filter((value) => value.id !== item.id))}
            />
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  )
}

export function useToast(): ToastContextValue {
  const context = useContext(ToastContext)
  if (!context) throw new Error('useToast must be used inside ToastProvider')
  return context
}

export function Pagination({
  hasPrevious,
  hasNext,
  onPrevious,
  onNext,
  previousLabel,
  nextLabel
}: {
  hasPrevious: boolean
  hasNext: boolean
  onPrevious: () => void
  onNext: () => void
  previousLabel: string
  nextLabel: string
}) {
  return (
    <nav className={styles.pagination} aria-label="Pagination">
      <Button size="small" disabled={!hasPrevious} onClick={onPrevious}>
        {previousLabel}
      </Button>
      <Button size="small" disabled={!hasNext} onClick={onNext}>
        {nextLabel}
      </Button>
    </nav>
  )
}
