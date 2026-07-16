import type { PlatformErrorCode } from "../../../../resources/generated/error-sdk/typescript/errorContract";

export type AdminSessionExpiryInput = {
  statusCode: number;
  requestToken: string;
  currentToken: string;
  errorCode?: PlatformErrorCode;
};

export function shouldExpireAdminSession({ statusCode, requestToken, currentToken, errorCode }: AdminSessionExpiryInput) {
  return statusCode === 401
    && errorCode !== "ADMIN_SENSITIVE_REVEAL_VERIFICATION_FAILED"
    && Boolean(requestToken)
    && currentToken === requestToken;
}
