import { useNavigate } from 'react-router-dom'
import { FileQuestionIcon } from 'lucide-react'
import { ErrorState } from '@/components/error-state'

export default function NotFoundPage() {
  const navigate = useNavigate()

  return (
    <ErrorState
      icon={<FileQuestionIcon />}
      title="Page not found"
      description="This URL doesn't match any page in sentry-lite. Check the address or head back to projects."
      onBack={() => navigate(-1)}
      homeTo="/"
      homeLabel="Go home"
    />
  )
}
