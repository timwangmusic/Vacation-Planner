import Alert from "react-bootstrap/Alert";

export function MyAlert({ variant, show, message }) {
  if (show) {
    return (
      <Alert variant={variant}>
        {message}
      </Alert>
    );
  }
  return null;
}
