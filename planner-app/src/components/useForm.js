import {useState} from 'react'

const useForm = (callback) => {
    const [values, setValues] = useState({})

    const handleSubmit = (event) => {
        if (event) {
            event.preventDefault()
        var proxyUrl = 'https://cors-anywhere.herokuapp.com/',
        targetUrl = 'https://best-vacation-planner.herokuapp.com/login'

        fetch ( proxyUrl+targetUrl, {
            method: 'POST',
            headers: {
		        'content-Type': 'application/json',
		        'Accept' : 'application/json',
		    },
            body: JSON.stringify(values)
        })
        .then((response) => {
            response.json()
            alert('Logged in..')
            })
        .then((values) => {
            console.log('Success:')
        })
        .catch((error) => {
            console.error('Error:', error)
    })
        callback()
    }
    }

    const handleChange = (event) => {
        event.persist()
        setValues(values => ({ ...values,
            [event.target.name]: event.target.value })
)}

    return {
        handleChange,
        handleSubmit,
        values,
    }
}

export default useForm
