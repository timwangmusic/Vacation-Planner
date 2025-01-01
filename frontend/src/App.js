import "bootstrap/dist/css/bootstrap.min.css";
import { Title } from "./title";
import "./App.css";
import { RequestForm } from "./requestForm";

function App() {
  return (
    <>
      <div className="App">
        <Title />
        <RequestForm />
      </div>
    </>
  );
}

export default App;
