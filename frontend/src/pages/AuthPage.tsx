import { useMemo } from 'react'
import { zodResolver } from '@hookform/resolvers/zod'
import { BookMarked, CircleAlert } from 'lucide-react'
import { useForm } from 'react-hook-form'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { useLogin, useRegister } from '../api/hooks'
import { getDeviceIdentity } from '../api/device'
import { ApiError } from '../api/http'
import { Button, Field, Input, Select } from '../shared/ui'
import styles from './pages.module.css'

interface AuthValues {
  display_name: string
  email: string
  password: string
  locale: 'ru' | 'en'
}

export function AuthPage({ mode }: { mode: 'login' | 'register' }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const login = useLogin()
  const registerUser = useRegister()
  const mutation = mode === 'login' ? login : registerUser
  const schema = useMemo(
    () =>
      z.object({
        display_name: mode === 'register' ? z.string().trim().min(2, t('auth.name')) : z.string(),
        email: z.string().trim().email(t('auth.email')),
        password: z.string().min(mode === 'register' ? 10 : 1, t('auth.passwordHint')),
        locale: z.enum(['ru', 'en'])
      }),
    [mode, t]
  )
  const {
    register,
    handleSubmit,
    formState: { errors }
  } = useForm<AuthValues>({
    resolver: zodResolver(schema),
    defaultValues: { display_name: '', email: '', password: '', locale: 'ru' }
  })

  const submit = handleSubmit(async (values) => {
    const device = getDeviceIdentity()
    if (mode === 'login') {
      await login.mutateAsync({ email: values.email, password: values.password, ...device })
    } else {
      await registerUser.mutateAsync({
        email: values.email,
        password: values.password,
        display_name: values.display_name,
        locale: values.locale,
        timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
        ...device
      })
    }
    const from = (location.state as { from?: string } | null)?.from
    void navigate(from || '/library', { replace: true })
  })

  const errorMessage =
    mutation.error instanceof ApiError && mutation.error.status !== 401
      ? mutation.error.message
      : t('auth.invalidCredentials')

  return (
    <div className={styles.authPage}>
      <section className={styles.authIntro} aria-labelledby="auth-welcome">
        <div className={styles.authBrand}>
          <span className={styles.authMark}>
            <BookMarked size={18} aria-hidden="true" />
          </span>
          <span>{t('common.appName')}</span>
        </div>
        <div className={styles.authCopy}>
          <h1 id="auth-welcome">{t('auth.welcome')}</h1>
          <p>{t('auth.subtitle')}</p>
        </div>
      </section>
      <main className={styles.authFormWrap}>
        <form className={styles.authForm} onSubmit={(event) => void submit(event)} noValidate>
          <h2>{mode === 'login' ? t('auth.login') : t('auth.register')}</h2>
          {mutation.isError ? (
            <div className={styles.formError} role="alert">
              <CircleAlert size={17} aria-hidden="true" />
              <span>{errorMessage}</span>
            </div>
          ) : null}
          {mode === 'register' ? (
            <Field
              label={t('auth.name')}
              htmlFor="display-name"
              error={errors.display_name?.message}
            >
              <Input
                id="display-name"
                autoComplete="name"
                aria-invalid={Boolean(errors.display_name)}
                {...register('display_name')}
              />
            </Field>
          ) : null}
          <Field label={t('auth.email')} htmlFor="email" error={errors.email?.message}>
            <Input
              id="email"
              type="email"
              autoComplete="email"
              inputMode="email"
              aria-invalid={Boolean(errors.email)}
              {...register('email')}
            />
          </Field>
          <Field
            label={t('auth.password')}
            htmlFor="password"
            hint={mode === 'register' ? t('auth.passwordHint') : undefined}
            error={errors.password?.message}
          >
            <Input
              id="password"
              type="password"
              autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
              aria-invalid={Boolean(errors.password)}
              {...register('password')}
            />
          </Field>
          {mode === 'register' ? (
            <Field label={t('common.selectLanguage')} htmlFor="locale">
              <Select id="locale" {...register('locale')}>
                <option value="ru">{t('common.russian')}</option>
                <option value="en">{t('common.english')}</option>
              </Select>
            </Field>
          ) : null}
          <Button type="submit" variant="accent" size="large" loading={mutation.isPending}>
            {mode === 'login' ? t('auth.login') : t('auth.register')}
          </Button>
          <p className={styles.authSwitch}>
            {mode === 'login' ? t('auth.noAccount') : t('auth.haveAccount')}{' '}
            <Link to={mode === 'login' ? '/register' : '/login'}>
              {mode === 'login' ? t('auth.register') : t('auth.login')}
            </Link>
          </p>
        </form>
      </main>
    </div>
  )
}
