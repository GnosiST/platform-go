type AuthProviderWithAudiences = {
  audiences: readonly string[];
};

const untrustedAuthorizationURLError = "OIDC authorization URL is not trusted";

export function filterAdminAuthProviders<T extends AuthProviderWithAudiences>(providers: readonly T[]) {
  return providers.filter((provider) => provider.audiences.includes("admin"));
}

export function assertAdminAuthProvider(provider: AuthProviderWithAudiences) {
  if (!provider.audiences.includes("admin")) {
    throw new Error("OIDC provider is not available for Admin login");
  }
}

export function validateOIDCAuthorizationURL(rawURL: string) {
  try {
    const authorizationURL = new URL(rawURL);
    if (authorizationURL.protocol === "https:") {
      return authorizationURL.toString();
    }
    if (authorizationURL.protocol === "http:" && isLoopbackHostname(authorizationURL.hostname)) {
      return authorizationURL.toString();
    }
  } catch {
    // Normalize malformed and untrusted values to one sanitized browser-boundary error.
  }
  throw new Error(untrustedAuthorizationURLError);
}

function isLoopbackHostname(hostname: string) {
  const normalizedHostname = hostname.toLowerCase();
  if (
    normalizedHostname === "localhost" ||
    normalizedHostname.endsWith(".localhost") ||
    normalizedHostname === "[::1]" ||
    normalizedHostname === "::1"
  ) {
    return true;
  }
  return /^127(?:\.\d{1,3}){3}$/u.test(normalizedHostname);
}
