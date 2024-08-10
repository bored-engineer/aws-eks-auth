package eksauth

import (
	"context"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"golang.org/x/oauth2"
)

// DefaultExpiration is the default expiration time for a generated EKS token.
var DefaultExpiration = 15 * time.Minute

// DefaultEarlyExpiry is the delta added to expire generated tokens early to account for clock skew.
var DefaultEarlyExpiry = 60 * time.Second

// wrappedSignerV4 extracts the expiration time of the credentials that were used to sign each request.
// If they will expire prior to the target time.Time, it replaces that value with the credential expiration.
type wrappedSignerV4 struct {
	target *time.Time
	signer sts.HTTPPresignerV4
}

// PresignHTTP implements the sts.HTTPPresignerV4 interface.
func (w *wrappedSignerV4) PresignHTTP(
	ctx context.Context, credentials aws.Credentials, r *http.Request,
	payloadHash string, service string, region string, signingTime time.Time,
	optFns ...func(*v4.SignerOptions),
) (signedURI string, signedHeaders http.Header, err error) {
	if credentials.CanExpire && !credentials.Expires.IsZero() {
		if credentials.Expires.Before(*w.target) {
			*w.target = credentials.Expires
		}
	}
	return w.signer.PresignHTTP(ctx, credentials, r, payloadHash, service, region, signingTime, optFns...)
}

// TokenSource is an oauth2.TokenSource that generates AWS EKS tokens from a sts.PresignClient.
// NOTE: Generally this should not be used directly, instead use the New* functions...
type TokenSource struct {
	ClusterName string
	Client      *sts.PresignClient
}

// Token implements the oauth2.TokenSource interface.
func (ts *TokenSource) Token() (*oauth2.Token, error) {
	expiry := time.Now().Add(DefaultExpiration)
	req, err := ts.Client.PresignGetCallerIdentity(
		context.TODO(),
		&sts.GetCallerIdentityInput{},
		func(opts *sts.PresignOptions) {
			opts.ClientOptions = []func(*sts.Options){
				sts.WithAPIOptions(
					smithyhttp.AddHeaderValue("X-K8s-Aws-Id", ts.ClusterName),
					smithyhttp.AddHeaderValue("X-Amz-Expires", "60"),
				),
			}
			opts.Presigner = &wrappedSignerV4{
				target: &expiry,
				signer: opts.Presigner,
			}
		},
	)
	if err != nil {
		return nil, err
	}
	return &oauth2.Token{
		AccessToken: "k8s-aws-v1." + base64.RawURLEncoding.EncodeToString([]byte(req.URL)),
		Expiry:      expiry,
	}, nil
}

// NewFromPresignClient creates a new oauth2.TokenSource from a sts.PresignClient and an EKS cluster name
func NewFromPresignClient(client *sts.PresignClient, clusterName string) oauth2.TokenSource {
	return oauth2.ReuseTokenSourceWithExpiry(nil, &TokenSource{
		ClusterName: clusterName,
		Client:      client,
	}, DefaultEarlyExpiry)
}

// NewFromClient creates a new oauth2.TokenSource from a sts.Client and an EKS cluster name
func NewFromClient(client *sts.Client, clusterName string) oauth2.TokenSource {
	return NewFromPresignClient(sts.NewPresignClient(client), clusterName)
}

// NewFromConfig creates a new oauth2.TokenSource from an aws.Config and an EKS cluster name
func NewFromConfig(cfg aws.Config, clusterName string) oauth2.TokenSource {
	return NewFromClient(sts.NewFromConfig(cfg), clusterName)
}
