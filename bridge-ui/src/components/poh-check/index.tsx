import clsx from "clsx";
import { useAccount, useSignMessage } from "wagmi";
import { Loader } from "@/components/ui/loader";
import { Dispatch, SetStateAction, useEffect, useRef, useState } from "react";
import { PohStep } from "@/components/modal-base/user-wallet";
import CloudCheck from "@/assets/icons/cloud-check.svg";
import CloudCheckOutline from "@/assets/icons/cloud-check-outline.svg";
import styles from "./poh-check.module.scss";
import { getAddress } from "viem";
import { linea } from "viem/chains";

type Props = {
  isHuman: boolean;
  refetchPoh: () => void;
  isCheckingPoh: boolean;
  setStep: Dispatch<SetStateAction<PohStep>>;
};

export default function PohCheck({ isHuman, refetchPoh, setStep, isCheckingPoh }: Props) {
  const { address } = useAccount();
  const [iframeUrl, setIframeUrl] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const { signMessage, data: signature, isPending: isPendingSignMessage } = useSignMessage();
  const iframeRef = useRef<HTMLIFrameElement>(null);
  const hasGeneratedMessageRef = useRef(false);
  const lastAddressRef = useRef<string | null>(null);

  const formattedAddress = getAddress(address ?? "0x0000000000000000000000000000000000000000");

  // Generate message only once when component mounts or address changes
  useEffect(() => {
    // Only generate if we haven't generated yet, or if the address has changed
    if (!hasGeneratedMessageRef.current || lastAddressRef.current !== formattedAddress) {
      const generatedMessage = generateSignInMessage(formattedAddress);
      setMessage(generatedMessage);
      hasGeneratedMessageRef.current = true;
      lastAddressRef.current = formattedAddress;
    }
  }, [formattedAddress]);

  const generateSignInMessage = (addr: string) => {
    const nonce = Math.random().toString(36).slice(2);
    const issuedAt = new Date().toISOString();
    return `${window.location.host} wants you to sign in with your Ethereum account:\n${addr}\n\nI confirm that I am the owner of this wallet and consent to performing a risk assessment and issuing a Verax attestation to this address.\n\nURI: https://in.sumsub.com\nVersion: 1\nChain ID: ${linea.id}\nNonce: ${nonce}\nIssued At: ${issuedAt}`;
  };

  const handleCardClick = () => {
    if (isHuman || !message) return;
    signMessage({ message });
  };

  useEffect(() => {
    if (signature && message) {
      const msg = btoa(
        JSON.stringify({
          signInMessage: message,
          signature,
        }),
      );

      const url = new URL("https://in.sumsub.com/websdk/p/uni_BKWTkQpZ2EqnGoY7");
      url.search = new URLSearchParams({
        utm_source: "utm_source",
        authMsg: msg,
      }).toString();

      setIframeUrl(url.toString());
      setStep(PohStep.SUMSUB_VERIFICATION);
    }
  }, [signature, message, setStep]);

  useEffect(() => {
    if (!iframeUrl || !iframeRef.current) return;

    const iframe = iframeRef.current;
    const ac = new AbortController();

    const handleMessage = (e: MessageEvent) => {
      if (!iframe) return;
      if (e.source !== iframe.contentWindow) return;
      if (e.origin !== "https://in.sumsub.com") return;
      if (e.data?.status === "completed") {
        refetchPoh();
        setStep(PohStep.IDLE);
        ac.abort();
      }
    };

    window.addEventListener("message", handleMessage, { signal: ac.signal });

    return () => ac.abort();
  }, [iframeUrl, refetchPoh, setStep]);

  if (iframeUrl)
    return (
      <iframe
        ref={iframeRef}
        id="sumsub-frame"
        src={iframeUrl}
        width="100%"
        height="700"
        allow="camera; microphone; geolocation"
        title="Sumsub Verification"
        style={{ border: "none" }}
      />
    );

  const getText = () => {
    if (isPendingSignMessage) return "Signing a message to confirm ownership";
    if (isCheckingPoh) return "Checking humanity";
    return isHuman ? "Humanity Verified" : "Verify Humanity";
  };
  const isLoading = isPendingSignMessage || isCheckingPoh;
  const cardClassName = clsx(styles.card, isHuman && styles.verified);

  const content = (
    <>
      {isLoading ? (
        <Loader className={styles.loader} fill="var(--color-indigo)" />
      ) : isHuman ? (
        <CloudCheck className={clsx(styles.icon, styles.verified)} />
      ) : (
        <CloudCheckOutline className={styles.icon} />
      )}
      <p className={styles.text}>{getText()}</p>
    </>
  );

  if (isHuman) {
    return <div className={cardClassName}>{content}</div>;
  }

  return (
    <button type="button" className={cardClassName} onClick={handleCardClick} disabled={isLoading}>
      {content}
    </button>
  );
}
