import type { VariantProps } from 'class-variance-authority'
import type { badgeVariants } from '@/components/ui/badge'

type BadgeVariant = NonNullable<VariantProps<typeof badgeVariants>['variant']>

export function statusVariant(status: string): BadgeVariant {
  switch (status) {
    case 'open':
      return 'destructive'
    case 'resolved':
      return 'default'
    case 'ignored':
      return 'secondary'
    default:
      return 'outline'
  }
}
