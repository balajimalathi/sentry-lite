"use client"

import { useState, useTransition } from "react"
import * as Sentry from "@sentry/nextjs"
import { BugIcon, ServerCrashIcon, CircleAlertIcon } from "lucide-react"

import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { captureServerException, triggerServerError } from "@/app/actions"

export function ErrorTriggers() {
  const [pending, startTransition] = useTransition()
  const [status, setStatus] = useState<string | null>(null)

  function onClientThrow() {
    setStatus(null)
    throw new Error("sample client throw (sentry-lite)")
  }

  function onClientCapture() {
    try {
      throw new Error("sample client captureException (sentry-lite)")
    } catch (e) {
      Sentry.captureException(e, {
        tags: { service: "sample", surface: "client" },
      })
      setStatus("Client exception sent. Check Issues for project 3.")
    }
  }

  function onServerThrow() {
    setStatus(null)
    startTransition(async () => {
      try {
        await triggerServerError()
      } catch {
        setStatus("Server action threw. Check Issues for project 3.")
      }
    })
  }

  function onServerCapture() {
    setStatus(null)
    startTransition(async () => {
      const result = await captureServerException()
      if (result?.ok) {
        setStatus("Server exception captured. Check Issues for project 3.")
      }
    })
  }

  return (
    <Card className="w-full max-w-lg">
      <CardHeader>
        <CardTitle>sentry-lite smoke test</CardTitle>
        <CardDescription>
          Send errors to project 3 via{" "}
          <code className="text-xs">@sentry/nextjs</code>. Open the triage UI
          afterward.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        <Alert>
          <CircleAlertIcon />
          <AlertTitle>Where to look</AlertTitle>
          <AlertDescription>
            Issues UI:{" "}
            <a
              href="http://localhost:5173"
              className="underline underline-offset-3 hover:text-foreground"
            >
              http://localhost:5173
            </a>{" "}
            (or{" "}
            <a
              href="http://localhost:8080"
              className="underline underline-offset-3 hover:text-foreground"
            >
              :8080
            </a>{" "}
            when serving the built UI).
          </AlertDescription>
        </Alert>
        {status ? (
          <Alert>
            <CircleAlertIcon />
            <AlertTitle>Sent</AlertTitle>
            <AlertDescription>{status}</AlertDescription>
          </Alert>
        ) : null}
      </CardContent>
      <CardFooter className="flex flex-wrap gap-2">
        <Button type="button" variant="destructive" onClick={onClientThrow}>
          <BugIcon data-icon="inline-start" />
          Client throw
        </Button>
        <Button type="button" variant="outline" onClick={onClientCapture}>
          <BugIcon data-icon="inline-start" />
          Client capture
        </Button>
        <Button
          type="button"
          variant="destructive"
          disabled={pending}
          onClick={onServerThrow}
        >
          <ServerCrashIcon data-icon="inline-start" />
          Server throw
        </Button>
        <Button
          type="button"
          variant="outline"
          disabled={pending}
          onClick={onServerCapture}
        >
          <ServerCrashIcon data-icon="inline-start" />
          Server capture
        </Button>
      </CardFooter>
    </Card>
  )
}
